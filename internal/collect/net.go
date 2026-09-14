package collect

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"

	"watcher/internal/store"
)

func (e *Engine) collectNet() error {
	conns, err := gnet.Connections("all")
	if err != nil {
		conns, err = gnet.Connections("tcp")
		if err != nil {
			return err
		}
	}
	now := time.Now().Unix()
	listen := []store.ListenPort{}
	agg := map[string]*ConnRow{}
	states := map[string]int{}
	seenListen := map[string]bool{}

	for _, c := range conns {
		if c.Family == 1 { // AF_UNIX
			continue
		}
		st := strings.ToUpper(c.Status)
		if st == "" {
			st = "NONE"
		}
		states[st]++
		lport := int(c.Laddr.Port)
		rport := int(c.Raddr.Port)
		lip := c.Laddr.IP
		rip := c.Raddr.IP
		proto := connFamily(c.Family, c.Type)

		if st == "LISTEN" || (c.Type == 2 && rport == 0 && rip == "") { // UDP socket
			if st != "LISTEN" && c.Type != 2 {
				continue
			}
			key := fmt.Sprintf("%s|%s|%d", proto, lip, lport)
			if seenListen[key] {
				continue
			}
			seenListen[key] = true
			name := ""
			if c.Pid > 0 {
				name = pidName(c.Pid)
			}
			pproto := proto
			if c.Type == 2 {
				pproto = "udp"
			} else {
				pproto = "tcp"
			}
			listen = append(listen, store.ListenPort{
				Proto: pproto, Addr: displayAddr(lip), Port: lport, PID: c.Pid, Name: name,
			})
			continue
		}
		if rip == "" || rip == "0.0.0.0" || rip == "::" || isLoopback(rip) {
			continue
		}
		key := fmt.Sprintf("%s|%d|%s", rip, lport, st)
		row := agg[key]
		if row == nil {
			row = &ConnRow{SrcIP: rip, DstPort: lport, State: st}
			agg[key] = row
		}
		row.Count++
	}

	born, err := e.store.UpsertListen(now, listen)
	if err != nil {
		return err
	}
	settings := e.store.LoadSettings(e.cfg.Settings)
	allow := map[string]bool{}
	for _, spec := range settings.ListenAllow {
		proto, port := splitSpec(spec)
		allow[proto+":"+port] = true
		allow["*:"+port] = true
	}
	if _, ok := e.store.GetMeta("listen_learned"); !ok {
		for _, p := range listen {
			spec := fmt.Sprintf("%s:%d", p.Proto, p.Port)
			if !containsSpec(settings.ListenAllow, spec) {
				settings.ListenAllow = append(settings.ListenAllow, spec)
			}
		}
		_ = e.store.SaveSettings(settings)
		_ = e.store.SetMeta("listen_learned", fmt.Sprintf("%d", now))
		allow = map[string]bool{}
		for _, spec := range settings.ListenAllow {
			proto, port := splitSpec(spec)
			allow[proto+":"+port] = true
		}
		born = nil
	}
	for i := range listen {
		_, ok := allow[fmt.Sprintf("%s:%d", listen[i].Proto, listen[i].Port)]
		listen[i].Allowed = ok
	}

	connsOut := make([]ConnRow, 0, len(agg))
	for _, r := range agg {
		connsOut = append(connsOut, *r)
	}
	sort.Slice(connsOut, func(i, j int) bool { return connsOut[i].Count > connsOut[j].Count })
	if len(connsOut) > 80 {
		connsOut = connsOut[:80]
	}

	snap := NetSnapshot{Ts: now, Listen: listen, Conns: connsOut, States: states}
	e.mu.Lock()
	e.net = snap
	e.recent = append(e.recent, snap)
	if len(e.recent) > 12 {
		e.recent = e.recent[len(e.recent)-12:]
	}
	e.mu.Unlock()

	for _, p := range born {
		if allow[fmt.Sprintf("%s:%d", p.Proto, p.Port)] {
			continue
		}
		ev := store.Event{
			Ts: now, Source: "net", Type: "new_listen", Severity: "high",
			Path:    fmt.Sprintf("%s:%d", p.Proto, p.Port),
			Message: fmt.Sprintf("新监听端口 %s:%d (%s)", p.Proto, p.Port, p.Name),
		}
		e.emit(ev)
	}
	return nil
}

func (e *Engine) RecentNets() []NetSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]NetSnapshot, len(e.recent))
	copy(out, e.recent)
	return out
}

func connFamily(family, typ uint32) string {
	if typ == 2 {
		return "udp"
	}
	return "tcp"
}

func displayAddr(ip string) string {
	if ip == "" || ip == "0.0.0.0" || ip == "::" || ip == "*" {
		return "*"
	}
	return ip
}

func isLoopback(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

func pidName(pid int32) string {
	b, err := osReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func splitSpec(spec string) (string, string) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if i := strings.IndexByte(spec, ':'); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return "tcp", spec
}

func containsSpec(list []string, spec string) bool {
	for _, s := range list {
		if strings.EqualFold(s, spec) {
			return true
		}
	}
	return false
}
