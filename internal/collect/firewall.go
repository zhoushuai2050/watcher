package collect

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"watcher/internal/store"
)

var reIPv4 = regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)

func (e *Engine) collectFirewall() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nft", "list", "sets").CombinedOutput()
	if err != nil {
		if _, look := exec.LookPath("nft"); look != nil {
			e.setStatus("firewall", true, true, "nft 不存在，已跳过")
			return nil
		}
		e.setStatus("firewall", true, true, "无权限执行 nft list，已跳过")
		return nil
	}
	sets := parseNftSets(out)
	e.mu.Lock()
	prev := e.banned
	e.banned = sets
	e.mu.Unlock()
	if prev == nil {
		e.setStatus("firewall", true, true, "已读取 nft 集合")
		return nil
	}
	for ip, set := range sets {
		if _, ok := prev[ip]; !ok {
			e.emit(store.Event{
				Ts: time.Now().Unix(), Source: "nft", Type: "fw_ban", Severity: "medium",
				SrcIP: ip, Path: set, Message: "防火墙集合新增 " + ip + " (" + set + ")",
			})
		}
	}
	e.setStatus("firewall", true, true, "已读取 nft 集合")
	return nil
}

func parseNftSets(raw []byte) map[string]string {
	out := map[string]string{}
	current := ""
	interesting := false
	for _, line := range bytes.Split(raw, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if strings.HasPrefix(s, "set ") {
			current = strings.TrimSpace(strings.TrimPrefix(s, "set "))
			low := strings.ToLower(current)
			interesting = strings.Contains(low, "ban") || strings.Contains(low, "block") || strings.Contains(low, "black") || strings.Contains(low, "deny") || strings.Contains(low, "drop")
			continue
		}
		if !interesting || current == "" {
			continue
		}
		for _, m := range reIPv4.FindAllString(s, -1) {
			out[m] = current
		}
	}
	return out
}
