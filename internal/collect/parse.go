package collect

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"watcher/internal/store"
)

var (
	reSSHFailed   = regexp.MustCompile(`Failed (?:password|none) for (invalid user )?(\S+) from (\S+) port (\d+)`)
	reSSHInvalid  = regexp.MustCompile(`Invalid user (\S+) from (\S+)`)
	reSSHAccepted = regexp.MustCompile(`Accepted (password|publickey|keyboard-interactive|passkey) for (\S+) from (\S+) port (\d+)`)
	reSSHTimeout  = regexp.MustCompile(`Timeout before authentication for connection from (\S+)`)
	reSSHMaxAuth  = regexp.MustCompile(`Maximum authentication attempts exceeded for .* from (\S+)`)
	reNginx       = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) ([^"]*) HTTP\/[^"]*" (\d{3})`)
	reFail2ban    = regexp.MustCompile(`\[([^\]]+)\] (Ban|Unban) (\S+)`)
	reKernDrop    = regexp.MustCompile(`(?i)(DROP|REJECT).*SRC=(\d+\.\d+\.\d+\.\d+)`)
	reProbePath   = regexp.MustCompile(`(?i)(/wp-login\.php|/xmlrpc\.php|/\.env\b|/\.git|/phpmyadmin|/wp-admin|/\.aws|/actuator|/vendor/phpunit|/cgi-bin|/HNAP1|/boaform|/setup\.cgi|/manager/html|/console|/solr|/geoserver|/jenkins|\.php7|/config\.json|/telescope|/server-status|/owa|/ecp|/wp-config)`)
	reLoginPath   = regexp.MustCompile(`(?i)(wp-login|xmlrpc|/signin|/login)`)
	reScannerUA   = regexp.MustCompile(`(?i)(nikto|sqlmap|masscan|zgrab|nmap|dirbuster|gobuster|wpscan|nessus|libwww-perl|httpx|nuclei)`)
)

func ParseSSHLine(line string) *store.Event {
	if strings.Contains(line, "pam_unix") || strings.Contains(line, "Connection closed by") {
		return nil
	}
	if !strings.Contains(line, "sshd") && !strings.Contains(line, "Failed ") && !strings.Contains(line, "Invalid user") && !strings.Contains(line, "Accepted ") && !strings.Contains(line, "Timeout before authentication") && !strings.Contains(line, "Maximum authentication") {
		return nil
	}
	ts := parseLineTime(line)
	if m := reSSHAccepted.FindStringSubmatch(line); m != nil {
		return &store.Event{
			Ts: ts, Source: "ssh", Type: "ssh_accepted", Severity: "info",
			SrcIP: sanitizeIP(m[3]), User: m[2],
			Message: "SSH 成功登录 (" + m[1] + ") 用户 " + m[2],
			Raw:     clip(line, 300),
		}
	}
	if m := reSSHFailed.FindStringSubmatch(line); m != nil {
		kind := "ssh_failed"
		if m[1] != "" {
			kind = "ssh_invalid"
		}
		return &store.Event{
			Ts: ts, Source: "ssh", Type: kind, Severity: "medium",
			SrcIP: sanitizeIP(m[3]), User: m[2],
			Message: "SSH 失败登录 用户 " + m[2],
			Raw:     clip(line, 300),
		}
	}
	if m := reSSHInvalid.FindStringSubmatch(line); m != nil {
		return &store.Event{
			Ts: ts, Source: "ssh", Type: "ssh_invalid", Severity: "medium",
			SrcIP: sanitizeIP(m[2]), User: m[1],
			Message: "SSH 无效用户 " + m[1],
			Raw:     clip(line, 300),
		}
	}
	if m := reSSHTimeout.FindStringSubmatch(line); m != nil {
		return &store.Event{
			Ts: ts, Source: "ssh", Type: "ssh_timeout", Severity: "low",
			SrcIP: sanitizeIP(m[1]), Message: "SSH 认证超时 " + m[1], Raw: clip(line, 300),
		}
	}
	if m := reSSHMaxAuth.FindStringSubmatch(line); m != nil {
		return &store.Event{
			Ts: ts, Source: "ssh", Type: "ssh_failed", Severity: "medium",
			SrcIP: sanitizeIP(m[1]), Message: "SSH 超过最大尝试次数", Raw: clip(line, 300),
		}
	}
	return nil
}

func ParseNginxLine(line string) *store.Event {
	m := reNginx.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	status, _ := strconv.Atoi(m[5])
	path := m[4]
	if i := strings.IndexByte(path, ' '); i > 0 {
		path = path[:i]
	}
	if q := strings.IndexByte(path, '?'); q > 0 {
		path = path[:q]
	}
	ip := sanitizeIP(m[1])
	ts := parseNginxTime(m[2])
	ua := ""
	if parts := strings.Split(line, `"`); len(parts) >= 6 {
		ua = parts[len(parts)-2]
	}
	probePath := reProbePath.MatchString(path)
	scanner := reScannerUA.MatchString(ua)
	probe := probePath || (scanner && !boringHTTPPath(path)) || (scanner && status >= 400 && status != 404)
	loginFail := reLoginPath.MatchString(path) && (status == 401 || status == 403)
	if !probe && !loginFail && status < 500 {
		return nil
	}
	ev := &store.Event{
		Ts: ts, Source: "nginx", SrcIP: ip, Path: path, Raw: clip(line, 300),
	}
	switch {
	case probe:
		ev.Type = "web_probe"
		ev.Severity = "medium"
		ev.Message = strconv.Itoa(status) + " 探测 " + path
	case loginFail:
		ev.Type = "web_failed"
		ev.Severity = "medium"
		ev.Message = strconv.Itoa(status) + " 登录失败 " + path
	default:
		if status >= 500 {
			ev.Type = "web_5xx"
			ev.Severity = "low"
			ev.Message = strconv.Itoa(status) + " " + m[3] + " " + path
		} else {
			return nil
		}
	}
	return ev
}

func ParseFail2banLine(line string) *store.Event {
	m := reFail2ban.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	kind := "fail2ban_ban"
	sev := "medium"
	msg := "Fail2ban 封禁 " + m[3] + " jail=" + m[1]
	if strings.EqualFold(m[2], "Unban") {
		kind = "fail2ban_unban"
		sev = "info"
		msg = "Fail2ban 解封 " + m[3] + " jail=" + m[1]
	}
	return &store.Event{
		Ts: parseLineTime(line), Source: "fail2ban", Type: kind, Severity: sev,
		SrcIP: sanitizeIP(m[3]), Path: m[1], Message: msg, Raw: clip(line, 300),
	}
}

func ParseKernDrop(line string) *store.Event {
	m := reKernDrop.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return &store.Event{
		Ts: parseLineTime(line), Source: "nft", Type: "fw_drop", Severity: "low",
		SrcIP: sanitizeIP(m[2]), Message: "防火墙丢弃来自 " + m[2], Raw: clip(line, 300),
	}
}

func boringHTTPPath(path string) bool {
	p := strings.ToLower(path)
	switch p {
	case "/", "/favicon.ico", "/robots.txt", "/index.html", "/index.htm":
		return true
	}
	for _, suf := range []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".woff", ".woff2"} {
		if strings.HasSuffix(p, suf) {
			return true
		}
	}
	return false
}

func parseLineTime(line string) int64 {
	line = strings.TrimSpace(line)
	now := time.Now()
	if line == "" {
		return now.Unix()
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05,000",
		"2006-01-02 15:04:05",
		"Jan 2 15:04:05",
		"Jan  2 15:04:05",
	}
	limit := len(line)
	if limit > 40 {
		limit = 40
	}
	for _, f := range formats {
		for n := limit; n >= 15; n-- {
			cand := strings.TrimSpace(line[:n])
			t, err := time.ParseInLocation(f, cand, time.Local)
			if err != nil {
				continue
			}
			if t.Year() < 2000 {
				t = t.AddDate(now.Year(), 0, 0)
				if t.Month() == time.December && now.Month() == time.January {
					t = t.AddDate(-1, 0, 0)
				}
			}
			return t.Unix()
		}
	}
	return now.Unix()
}

func parseNginxTime(s string) int64 {
	t, err := time.ParseInLocation("02/Jan/2006:15:04:05 -0700", s, time.Local)
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
}

func sanitizeIP(s string) string {
	s = strings.Trim(s, "[]")
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	return ip.String()
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func discoverAuthLog(configured string) string {
	if configured != "" {
		if _, err := osStat(configured); err == nil {
			return configured
		}
	}
	for _, p := range []string{"/var/log/auth.log", "/var/log/secure"} {
		if _, err := osStat(p); err == nil {
			return p
		}
	}
	return configured
}
