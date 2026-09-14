package detect

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"watcher/internal/collect"
	"watcher/internal/config"
	"watcher/internal/store"
)

type Detector struct {
	store      *store.Store
	cfg        config.Config
	engine     *collect.Engine
	hostBreach map[string]int64
}

func New(st *store.Store, cfg config.Config, engine *collect.Engine) *Detector {
	return &Detector{store: st, cfg: cfg, engine: engine, hostBreach: map[string]int64{}}
}

func (d *Detector) Start(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	d.Tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.Tick()
		}
	}
}

const attackBanWindow = 7 * 86400

func (d *Detector) OnEvent(ev store.Event) {
	if ev.Ts > 0 && time.Now().Unix()-ev.Ts > 24*3600 {
		return
	}
	s := d.store.LoadSettings(d.cfg.Settings)
	switch ev.Type {
	case "fail2ban_ban":
		_ = d.store.RaiseAlert("fail2ban.ban", ev.SrcIP, "medium", ev.Message)
	case "fail2ban_unban":
		_ = d.store.ResolveRuleKey("fail2ban.ban", ev.SrcIP)
	case "ssh_accepted":
		if ev.SrcIP == "" {
			return
		}
		fails, _ := d.store.CountFailsBefore(ev.SrcIP, ev.Ts-int64(s.SSHFailWindowSec), ev.Ts)
		if fails >= 3 {
			_ = d.store.RaiseAlert("ssh.success_after_fail", ev.SrcIP, "critical",
				fmt.Sprintf("IP %s 在 %d 次失败后以用户 %s 登录成功", ev.SrcIP, fails, ev.User))
		}
		known, err := d.store.KnownSSH(ev.SrcIP)
		if err == nil && !known {
			_ = d.store.RaiseAlert("ssh.new_source", ev.SrcIP, "high",
				fmt.Sprintf("新来源 SSH 登录 %s 用户 %s", ev.SrcIP, ev.User))
		}
		_ = d.store.RememberSSH(ev.SrcIP, ev.User)
	}
	if store.IsAttackFail(ev.Type) && ev.SrcIP != "" {
		d.maybePermanentBan(ev.SrcIP, s)
	}
}

func (d *Detector) Tick() {
	if err := d.tickOnce(); err != nil {
		log.Printf("detect: %v", err)
	}
}

func (d *Detector) tickOnce() error {
	s := d.store.LoadSettings(d.cfg.Settings)
	now := time.Now().Unix()

	if err := d.window("ssh", []string{"ssh_failed", "ssh_invalid", "ssh_timeout"}, s.SSHFailWindowSec, s.SSHFailCount, "ssh.bruteforce", "high",
		func(ip string, n int) string {
			return fmt.Sprintf("SSH 爆破 %s：%d 次失败 / %ds", ip, n, s.SSHFailWindowSec)
		}); err != nil {
		return err
	}
	if err := d.window("ssh", []string{"ssh_invalid"}, s.SSHFailWindowSec, s.InvalidUserCount, "ssh.invalid_user", "medium",
		func(ip string, n int) string { return fmt.Sprintf("SSH 扫无效用户 %s：%d 次", ip, n) }); err != nil {
		return err
	}
	if err := d.window("nginx", []string{"web_probe"}, s.WebProbeWindowSec, s.WebProbeCount, "web.probe", "medium",
		func(ip string, n int) string { return fmt.Sprintf("Web 路径探测 %s：%d 次", ip, n) }); err != nil {
		return err
	}
	if err := d.window("nginx", []string{"web_failed"}, s.WebProbeWindowSec, s.WebFailCount, "web.bruteforce", "high",
		func(ip string, n int) string { return fmt.Sprintf("Web 登录爆破 %s：%d 次", ip, n) }); err != nil {
		return err
	}

	host, procs, _, net := d.engine.Snapshot()
	d.checkHost(host, s)
	d.checkProc(procs, s)
	d.checkNet(net, s, now)
	d.banOffenders(s)
	return nil
}

func (d *Detector) maybePermanentBan(ip string, s config.Settings) {
	if s.AttackBanThreshold <= 0 || d.engine == nil {
		return
	}
	from := time.Now().Unix() - attackBanWindow
	n, err := d.store.CountAttackFailsIP(ip, from)
	if err != nil || n < s.AttackBanThreshold {
		return
	}
	reason := fmt.Sprintf("7 天内失败类事件 %d（阈值 %d）", n, s.AttackBanThreshold)
	if err := d.engine.BanPermanent(ip, reason, true); err != nil {
		log.Printf("detect ban %s: %v", ip, err)
	}
}

func (d *Detector) banOffenders(s config.Settings) {
	if s.AttackBanThreshold <= 0 || d.engine == nil {
		return
	}
	from := time.Now().Unix() - attackBanWindow
	rows, err := d.store.CountAttackFails(from)
	if err != nil {
		log.Printf("detect ban count: %v", err)
		return
	}
	for _, r := range rows {
		if r.Count >= s.AttackBanThreshold && r.IP != "" {
			reason := fmt.Sprintf("7 天内失败类事件 %d（阈值 %d）", r.Count, s.AttackBanThreshold)
			if err := d.engine.BanPermanent(r.IP, reason, true); err != nil {
				log.Printf("detect ban %s: %v", r.IP, err)
			}
		}
	}
}

func (d *Detector) window(source string, types []string, win, thresh int, rule, sev string, summary func(string, int) string) error {
	if thresh <= 0 || win <= 0 {
		return nil
	}
	from := time.Now().Unix() - int64(win)
	rows, err := d.store.CountEvents(source, types, from)
	if err != nil {
		return err
	}
	offenders := map[string]string{}
	for _, r := range rows {
		if r.Count >= thresh && r.IP != "" {
			offenders[r.IP] = summary(r.IP, r.Count)
		}
	}
	return d.raiseOrClear(rule, sev, offenders)
}

func (d *Detector) raiseOrClear(rule, sev string, offenders map[string]string) error {
	open, err := d.store.OpenAlerts(rule)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for key, summary := range offenders {
		if err := d.store.RaiseAlert(rule, key, sev, summary); err != nil {
			return err
		}
		seen[key] = true
	}
	for _, a := range open {
		if !seen[a.Key] {
			_ = d.store.ResolveAlert(a.ID)
		}
	}
	return nil
}

func (d *Detector) checkHost(h collect.HostSample, s config.Settings) {
	offCPU := map[string]string{}
	offMem := map[string]string{}
	offDisk := map[string]string{}
	if d.sustained("cpu", h.CPUPct >= s.CPUPct, s.CPUSustainSec) {
		offCPU["cpu"] = fmt.Sprintf("CPU 使用率 %.0f%% ≥ %.0f%% 持续 %ds", h.CPUPct, s.CPUPct, s.CPUSustainSec)
	}
	memPct := 0.0
	if h.MemTotal > 0 {
		memPct = float64(h.MemUsed) / float64(h.MemTotal) * 100
	}
	if d.sustained("mem", memPct >= s.MemPct && h.MemTotal > 0, s.CPUSustainSec) {
		offMem["mem"] = fmt.Sprintf("内存使用率 %.0f%% ≥ %.0f%% 持续 %ds", memPct, s.MemPct, s.CPUSustainSec)
	}
	if d.sustained("disk", h.DiskUsedPct >= s.DiskPct, s.CPUSustainSec) {
		offDisk["disk"] = fmt.Sprintf("根磁盘使用率 %.0f%% ≥ %.0f%% 持续 %ds", h.DiskUsedPct, s.DiskPct, s.CPUSustainSec)
	}
	_ = d.raiseOrClear("host.cpu", "medium", offCPU)
	_ = d.raiseOrClear("host.mem", "medium", offMem)
	_ = d.raiseOrClear("host.disk", "medium", offDisk)
}

func (d *Detector) sustained(key string, over bool, sec int) bool {
	if !over {
		delete(d.hostBreach, key)
		return false
	}
	if sec <= 0 {
		return true
	}
	now := time.Now().Unix()
	if d.hostBreach[key] == 0 {
		d.hostBreach[key] = now
		return false
	}
	return now-d.hostBreach[key] >= int64(sec)
}

func (d *Detector) checkProc(procs []store.ProcRow, s config.Settings) {
	off := map[string]string{}
	for _, p := range procs {
		if strings.EqualFold(p.Name, "watcher") {
			continue
		}
		if p.CPU >= s.ProcCPUPct {
			key := fmt.Sprintf("%s:%d", p.Name, p.PID)
			off[key] = fmt.Sprintf("进程 %s (pid %d) CPU %.0f%%", p.Name, p.PID, p.CPU)
			break
		}
	}
	_ = d.raiseOrClear("proc.cpu_hog", "low", off)
}

func (d *Detector) checkNet(net collect.NetSnapshot, s config.Settings, now int64) {
	flood := map[string]string{}
	for _, c := range net.Conns {
		if c.Count >= s.ConnFloodCount && (c.State == "ESTABLISHED" || c.State == "SYN_RECV" || c.State == "SYN_SENT") {
			key := fmt.Sprintf("%s:%d", c.SrcIP, c.DstPort)
			flood[key] = fmt.Sprintf("%s 对端口 %d 有 %d 条 %s 连接", c.SrcIP, c.DstPort, c.Count, c.State)
		}
	}
	_ = d.raiseOrClear("net.conn_flood", "high", flood)

	syn := map[string]string{}
	if n := net.States["SYN_RECV"]; n >= s.SynRecvCount && s.SynRecvCount > 0 {
		syn["syn"] = fmt.Sprintf("SYN_RECV 堆积 %d（阈值 %d）", n, s.SynRecvCount)
	}
	_ = d.raiseOrClear("net.syn_backlog", "high", syn)

	scans := d.portScans(s)
	_ = d.raiseOrClear("net.port_scan", "high", scans)

	allow := map[string]bool{}
	for _, spec := range s.ListenAllow {
		proto, port := splitSpec(spec)
		allow[proto+":"+port] = true
	}
	listenOff := map[string]string{}
	for _, p := range net.Listen {
		key := fmt.Sprintf("%s:%d", p.Proto, p.Port)
		if allow[key] {
			continue
		}
		listenOff[key] = fmt.Sprintf("未在白名单的监听 %s:%d (%s)", p.Proto, p.Port, p.Name)
	}
	_ = d.raiseOrClear("net.new_listen", "high", listenOff)
	_ = now
}

func (d *Detector) portScans(s config.Settings) map[string]string {
	out := map[string]string{}
	if d.engine == nil {
		return out
	}
	snaps := d.engine.RecentNets()
	if len(snaps) == 0 {
		return out
	}
	cutoff := time.Now().Unix() - int64(s.PortScanWindowSec)
	ports := map[string]map[int]bool{}
	for _, snap := range snaps {
		if snap.Ts < cutoff {
			continue
		}
		for _, c := range snap.Conns {
			if c.SrcIP == "" {
				continue
			}
			if ports[c.SrcIP] == nil {
				ports[c.SrcIP] = map[int]bool{}
			}
			ports[c.SrcIP][c.DstPort] = true
		}
	}
	for ip, set := range ports {
		if len(set) >= s.PortScanPorts {
			out[ip] = fmt.Sprintf("%s 在 %ds 内连接了 %d 个不同端口", ip, s.PortScanWindowSec, len(set))
		}
	}
	return out
}

func splitSpec(spec string) (string, string) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if i := strings.IndexByte(spec, ':'); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return "tcp", spec
}
