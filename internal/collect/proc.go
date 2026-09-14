package collect

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/process"

	"watcher/internal/store"
)

func (e *Engine) collectProc() error {
	list, err := process.Processes()
	if err != nil {
		return err
	}
	rows := make([]store.ProcRow, 0, len(list))
	for _, p := range list {
		name, _ := p.Name()
		if name == "" {
			continue
		}
		cpuPct, _ := p.CPUPercent()
		var rss int64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			rss = int64(mi.RSS)
		}
		user, _ := p.Username()
		cmd, _ := p.Cmdline()
		if len(cmd) > 200 {
			cmd = cmd[:200]
		}
		var rb, wb int64
		if io, err := p.IOCounters(); err == nil && io != nil {
			rb = int64(io.ReadBytes)
			wb = int64(io.WriteBytes)
		}
		unit, container := parseCgroup(p.Pid)
		rows = append(rows, store.ProcRow{
			PID: p.Pid, Name: name, Unit: unit, Container: container, User: user,
			CPU: cpuPct, RSS: rss, ReadBytes: rb, WriteBytes: wb, Cmdline: cmd,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CPU > rows[j].CPU })
	top := rows
	if len(top) > 30 {
		top = top[:30]
	}
	ts := nowUnix()
	if err := e.store.InsertProcesses(ts, top); err != nil {
		return err
	}
	groups := groupProcs(rows)
	e.mu.Lock()
	e.procs = top
	e.groups = groups
	e.mu.Unlock()
	return nil
}

func groupProcs(rows []store.ProcRow) []ProcGroup {
	type acc struct {
		cpu       float64
		rss       int64
		n         int
		kind, key string
	}
	m := map[string]*acc{}
	for _, r := range rows {
		kind, key := "name", r.Name
		if r.Container != "" {
			kind, key = "container", r.Container
		} else if r.Unit != "" && !strings.HasPrefix(r.Unit, "user-") && r.Unit != "init.scope" {
			kind, key = "unit", r.Unit
		}
		a := m[kind+"|"+key]
		if a == nil {
			a = &acc{kind: kind, key: key}
			m[kind+"|"+key] = a
		}
		a.cpu += r.CPU
		a.rss += r.RSS
		a.n++
	}
	out := make([]ProcGroup, 0, len(m))
	for _, a := range m {
		out = append(out, ProcGroup{Key: a.key, Kind: a.kind, CPU: a.cpu, RSS: a.rss, Count: a.n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CPU > out[j].CPU })
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func parseCgroup(pid int32) (unit, container string) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		path := line
		if i := strings.IndexByte(line, ':'); i >= 0 {
			rest := line[i+1:]
			if j := strings.IndexByte(rest, ':'); j >= 0 {
				path = rest[j+1:]
			} else {
				path = rest
			}
		}
		if i := strings.Index(path, "docker-"); i >= 0 {
			id := path[i+7:]
			id = strings.TrimSuffix(id, ".scope")
			if len(id) > 12 {
				id = id[:12]
			}
			return "", "docker:" + id
		}
		if i := strings.Index(path, "/docker/"); i >= 0 {
			id := strings.TrimPrefix(path[i+8:], "")
			if slash := strings.IndexByte(id, '/'); slash >= 0 {
				id = id[:slash]
			}
			if len(id) > 12 {
				id = id[:12]
			}
			return "", "docker:" + id
		}
		if strings.Contains(path, "system.slice/") {
			rest := path[strings.Index(path, "system.slice/")+len("system.slice/"):]
			if slash := strings.IndexByte(rest, '/'); slash >= 0 {
				rest = rest[:slash]
			}
			return rest, ""
		}
	}
	return "", ""
}
