package collect

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"watcher/internal/config"
	"watcher/internal/store"
)

type Engine struct {
	cfg     config.Config
	store   *store.Store
	OnEvent func(store.Event)

	mu      sync.Mutex
	host    HostSample
	procs   []store.ProcRow
	groups  []ProcGroup
	net     NetSnapshot
	recent  []NetSnapshot
	banned  map[string]string
	status  map[string]Status
	netPrev ioSnap
}

func New(st *store.Store, cfg config.Config) *Engine {
	return &Engine{
		cfg:    cfg,
		store:  st,
		status: map[string]Status{},
		banned: nil,
	}
}

func (e *Engine) Start(ctx context.Context) {
	go e.loop(ctx, "host", 5*time.Second, e.collectHost)
	go e.loop(ctx, "proc", 10*time.Second, e.collectProc)
	go e.loop(ctx, "net", 10*time.Second, e.collectNet)
	go e.loop(ctx, "firewall", 30*time.Second, e.collectFirewall)
	go e.loop(ctx, "ban", 30*time.Second, e.syncBansLoop)
	go e.loop(ctx, "prune", 10*time.Minute, func() error {
		_ = e.store.Downsample()
		return e.store.Prune()
	})
	go e.runTail(ctx, "ssh", e.sshPath, ParseSSHLine)
	go e.runTail(ctx, "nginx", func() string {
		s := e.store.LoadSettings(e.cfg.Settings)
		return s.NginxLog
	}, ParseNginxLine)
	go e.runTail(ctx, "fail2ban", func() string {
		s := e.store.LoadSettings(e.cfg.Settings)
		return s.Fail2banLog
	}, ParseFail2banLine)
	go e.pollJournal(ctx)
	go e.runTail(ctx, "kern", func() string { return e.cfg.KernLog }, ParseKernDrop)
}

func (e *Engine) sshPath() string {
	s := e.store.LoadSettings(e.cfg.Settings)
	return discoverAuthLog(s.AuthLog)
}

func (e *Engine) loop(ctx context.Context, name string, every time.Duration, fn func() error) {
	e.setStatus(name, true, true, "starting")
	run := func() {
		err := fn()
		if err != nil {
			log.Printf("collector %s: %v", name, err)
			e.setStatus(name, true, false, err.Error())
			return
		}
		e.touchOK(name)
	}
	run()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

func (e *Engine) runTail(ctx context.Context, name string, pathFn func() string, parse func(string) *store.Event) {
	for {
		if ctx.Err() != nil {
			return
		}
		path := pathFn()
		if path == "" {
			e.setStatus(name, false, true, "未配置路径")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}
		e.setStatus(name, true, true, readable(path))
		tail := &Tailer{Path: path, StateKey: "tail:" + name, Store: e.store}
		err := tail.Follow(ctx, func(line string) {
			ev := parse(line)
			if ev != nil {
				e.emit(*ev)
			}
		})
		if ctx.Err() != nil {
			return
		}
		msg := readable(path)
		if err != nil {
			msg = err.Error()
		}
		e.setStatus(name, true, false, msg)
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (e *Engine) pollJournal(ctx context.Context) {
	if discoverAuthLog("") != "" {
		return
	}
	if _, err := exec.LookPath("journalctl"); err != nil {
		e.setStatus("ssh", true, false, "没有 auth.log，也没有 journalctl")
		return
	}
	e.setStatus("ssh", true, true, "journalctl -u ssh -u sshd")
	cursor := ""
	if v, ok := e.store.GetMeta("journal:ssh"); ok {
		cursor = v
	}
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			args := []string{"-u", "ssh", "-u", "sshd", "-u", "ssh.service", "--no-pager", "-o", "short-iso", "--show-cursor"}
			if cursor != "" {
				args = append(args, "--after-cursor", cursor)
			} else {
				args = append(args, "-n", "200")
			}
			out, err := exec.CommandContext(ctx, "journalctl", args...).CombinedOutput()
			if err != nil {
				e.setStatus("ssh", true, false, "journalctl: "+err.Error())
				continue
			}
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "-- cursor:") || strings.HasPrefix(line, "-- Cursor:") {
					cursor = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-- cursor:"), "-- Cursor:"))
					continue
				}
				if ev := ParseSSHLine(line); ev != nil {
					e.emit(*ev)
				}
			}
			if cursor != "" {
				_ = e.store.SetMeta("journal:ssh", cursor)
			}
			e.setStatus("ssh", true, true, "journalctl")
		}
	}
}

func (e *Engine) emit(ev store.Event) {
	if _, err := e.store.InsertEvent(ev); err != nil {
		log.Printf("insert event: %v", err)
		return
	}
	if e.OnEvent != nil {
		e.OnEvent(ev)
	}
}

func (e *Engine) touchOK(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.status[name]
	st.Name = name
	st.Enabled = true
	st.OK = true
	st.LastRun = time.Now().Unix()
	st.LastError = ""
	if st.Detail == "starting" {
		st.Detail = ""
	}
	e.status[name] = st
}

func (e *Engine) setStatus(name string, enabled, ok bool, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.status[name]
	st.Name = name
	st.Enabled = enabled
	st.OK = ok
	st.LastRun = time.Now().Unix()
	st.Detail = detail
	if ok {
		st.LastError = ""
	} else {
		st.LastError = detail
	}
	e.status[name] = st
}

func (e *Engine) Status() []Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := []string{"host", "proc", "net", "ssh", "nginx", "fail2ban", "firewall", "ban", "kern", "prune"}
	out := make([]Status, 0, len(names))
	for _, n := range names {
		if st, ok := e.status[n]; ok {
			out = append(out, st)
		}
	}
	return out
}

func (e *Engine) Snapshot() (HostSample, []store.ProcRow, []ProcGroup, NetSnapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.host, append([]store.ProcRow{}, e.procs...), append([]ProcGroup{}, e.groups...), e.net
}

func nowUnix() int64 { return time.Now().Unix() }
