package collect

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"watcher/internal/store"
)

var runNft = defaultRunNft

func defaultRunNft(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	bin, err := exec.LookPath("nft")
	if err != nil {
		return "", fmt.Errorf("nft 不存在")
	}
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return string(out), fmt.Errorf("%s", msg)
	}
	return string(out), nil
}

func SetRunNft(fn func(args ...string) (string, error)) func() {
	old := runNft
	if fn == nil {
		runNft = defaultRunNft
	} else {
		runNft = fn
	}
	return func() { runNft = old }
}

func nftExistsErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "file exists") || strings.Contains(s, "already exists")
}

func (e *Engine) syncBansLoop() error {
	if err := e.ensureBanTable(); err != nil {
		return err
	}
	if err := e.applyStoredBans(); err != nil {
		return err
	}
	if err := e.applyStoredAllows(); err != nil {
		return err
	}
	n, err := e.store.CountBans()
	if err != nil {
		return err
	}
	e.setStatus("ban", true, true, fmt.Sprintf("永久封禁 %d 个 IP", n))
	return nil
}

func (e *Engine) ensureBanTable() error {
	if _, err := runNft("add", "table", "inet", "watcher"); err != nil && !nftExistsErr(err) {
		return fmt.Errorf("nft add table: %w", err)
	}
	cmds := [][]string{
		{"add", "set", "inet", "watcher", "permanent4", "{ type ipv4_addr; }"},
		{"add", "set", "inet", "watcher", "permanent6", "{ type ipv6_addr; }"},
		{"add", "set", "inet", "watcher", "allow4", "{ type ipv4_addr; flags interval; }"},
		{"add", "set", "inet", "watcher", "allow6", "{ type ipv6_addr; flags interval; }"},
		{"add", "chain", "inet", "watcher", "input", "{ type filter hook input priority -2; policy accept; }"},
	}
	for _, c := range cmds {
		if _, err := runNft(c...); err != nil && !nftExistsErr(err) {
			return fmt.Errorf("nft %s: %w", strings.Join(c, " "), err)
		}
	}
	out, err := runNft("list", "chain", "inet", "watcher", "input")
	if err != nil || !strings.Contains(out, "@allow4") || !strings.Contains(out, "@permanent4") {
		return e.rebuildInputChain()
	}
	return nil
}

func (e *Engine) rebuildInputChain() error {
	if _, err := runNft("flush", "chain", "inet", "watcher", "input"); err != nil {
		return fmt.Errorf("nft flush chain: %w", err)
	}
	rules := [][]string{
		{"add", "rule", "inet", "watcher", "input", "ip", "saddr", "@allow4", "accept"},
		{"add", "rule", "inet", "watcher", "input", "ip6", "saddr", "@allow6", "accept"},
		{"add", "rule", "inet", "watcher", "input", "ip", "saddr", "@permanent4", "drop"},
		{"add", "rule", "inet", "watcher", "input", "ip6", "saddr", "@permanent6", "drop"},
	}
	for _, c := range rules {
		if _, err := runNft(c...); err != nil {
			return fmt.Errorf("nft %s: %w", strings.Join(c, " "), err)
		}
	}
	return nil
}

func (e *Engine) applyStoredBans() error {
	bans, err := e.store.ListBans()
	if err != nil {
		return err
	}
	var v4, v6 []string
	for _, b := range bans {
		ip, ok := NormalizeIP(b.IP)
		if !ok {
			continue
		}
		if strings.Contains(ip, ":") {
			v6 = append(v6, ip)
		} else {
			v4 = append(v4, ip)
		}
	}
	if err := replaceSet("permanent4", v4); err != nil {
		return err
	}
	return replaceSet("permanent6", v6)
}

func replaceSet(set string, ips []string) error {
	if _, err := runNft("flush", "set", "inet", "watcher", set); err != nil {
		return fmt.Errorf("nft flush %s: %w", set, err)
	}
	if len(ips) == 0 {
		return nil
	}
	_, err := runNft("add", "element", "inet", "watcher", set, "{ "+strings.Join(ips, ", ")+" }")
	if err != nil && !nftExistsErr(err) {
		return fmt.Errorf("nft add %s: %w", set, err)
	}
	return nil
}

func nftSetFor(ip string) string {
	if strings.Contains(ip, ":") {
		return "permanent6"
	}
	return "permanent4"
}

func nftAllowSet(spec string) string {
	if strings.Contains(spec, ":") {
		return "allow6"
	}
	return "allow4"
}

func (e *Engine) applyStoredAllows() error {
	rows, err := e.store.ListAllow()
	if err != nil {
		return err
	}
	var v4, v6 []string
	for _, a := range rows {
		if strings.Contains(a.Spec, ":") {
			v6 = append(v6, a.Spec)
		} else {
			v4 = append(v4, a.Spec)
		}
	}
	if err := replaceSet("allow4", v4); err != nil {
		return err
	}
	return replaceSet("allow6", v6)
}

func (e *Engine) AddAllow(spec, note string) error {
	norm, ok := store.NormalizeAllow(spec)
	if !ok {
		return fmt.Errorf("无效 IP / CIDR")
	}
	if err := e.store.InsertAllow(store.IPAllow{Spec: norm, Note: strings.TrimSpace(note)}); err != nil {
		return err
	}
	bans, err := e.store.ListBans()
	if err != nil {
		return err
	}
	for _, b := range bans {
		if store.AllowContains(norm, b.IP) {
			_ = e.UnbanPermanent(b.IP)
		}
	}
	if err := e.ensureBanTable(); err != nil {
		return err
	}
	if _, err := runNft("add", "element", "inet", "watcher", nftAllowSet(norm), "{ "+norm+" }"); err != nil && !nftExistsErr(err) {
		e.setStatus("ban", true, false, err.Error())
		return err
	}
	e.emit(store.Event{
		Ts: time.Now().Unix(), Source: "watcher", Type: "ip_allow", Severity: "info",
		SrcIP: norm, Message: "加入白名单 " + norm,
	})
	return nil
}

func (e *Engine) RemoveAllow(spec string) error {
	norm, ok := store.NormalizeAllow(spec)
	if !ok {
		norm = strings.TrimSpace(spec)
	}
	if err := e.store.DeleteAllow(norm); err != nil {
		return err
	}
	if err := e.ensureBanTable(); err != nil {
		return err
	}
	if _, err := runNft("delete", "element", "inet", "watcher", nftAllowSet(norm), "{ "+norm+" }"); err != nil {
		s := strings.ToLower(err.Error())
		if !strings.Contains(s, "no such file") && !strings.Contains(s, "not found") && !strings.Contains(s, "no such") {
			e.setStatus("ban", true, false, err.Error())
			return err
		}
	}
	e.emit(store.Event{
		Ts: time.Now().Unix(), Source: "watcher", Type: "ip_allow_del", Severity: "info",
		SrcIP: norm, Message: "移出白名单 " + norm,
	})
	return nil
}

func (e *Engine) BanPermanent(ip, reason string, auto bool) error {
	norm, ok := NormalizeIP(ip)
	if !ok {
		return fmt.Errorf("无效 IP")
	}
	if IsLoopbackIP(norm) {
		return fmt.Errorf("不能封禁本机地址")
	}
	if allowed, err := e.store.IsAllowed(norm); err != nil {
		return err
	} else if allowed {
		if auto {
			return nil
		}
		return fmt.Errorf("该 IP 在白名单中，不能封禁")
	}
	if auto && IsExemptIP(norm, nil) {
		return nil
	}
	hits, _ := e.store.CountAttackFailsIP(norm, time.Now().Unix()-7*86400)
	created, err := e.store.InsertBan(store.IPBan{
		IP:     norm,
		Reason: reason,
		Hits:   hits,
		Auto:   auto,
	})
	if err != nil {
		return err
	}
	if err := e.ensureBanTable(); err != nil {
		e.setStatus("ban", true, false, err.Error())
		return err
	}
	if _, err := runNft("add", "element", "inet", "watcher", nftSetFor(norm), "{ "+norm+" }"); err != nil && !nftExistsErr(err) {
		e.setStatus("ban", true, false, err.Error())
		return err
	}
	if created {
		e.emit(store.Event{
			Ts: time.Now().Unix(), Source: "watcher", Type: "ip_ban", Severity: "critical",
			SrcIP: norm, Message: "永久封禁 " + norm + "：" + reason,
		})
		_ = e.store.RaiseAlert("ip.permanent_ban", norm, "critical", "永久封禁 "+norm+"："+reason)
	}
	n, _ := e.store.CountBans()
	e.setStatus("ban", true, true, fmt.Sprintf("永久封禁 %d 个 IP", n))
	return nil
}

func (e *Engine) UnbanPermanent(ip string) error {
	norm, ok := NormalizeIP(ip)
	if !ok {
		return fmt.Errorf("无效 IP")
	}
	if err := e.store.DeleteBan(norm); err != nil {
		return err
	}
	_ = e.store.ResolveRuleKey("ip.permanent_ban", norm)
	if _, err := runNft("delete", "element", "inet", "watcher", nftSetFor(norm), "{ "+norm+" }"); err != nil {
		s := strings.ToLower(err.Error())
		if !strings.Contains(s, "no such file") && !strings.Contains(s, "not found") && !strings.Contains(s, "no such") {
			e.setStatus("ban", true, false, err.Error())
			return err
		}
	}
	e.emit(store.Event{
		Ts: time.Now().Unix(), Source: "watcher", Type: "ip_unban", Severity: "info",
		SrcIP: norm, Message: "解除永久封禁 " + norm,
	})
	n, _ := e.store.CountBans()
	e.setStatus("ban", true, true, fmt.Sprintf("永久封禁 %d 个 IP", n))
	return nil
}
