package collect

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func testdata(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func TestParseSSHLog(t *testing.T) {
	raw, err := os.ReadFile(testdata("auth.log"))
	if err != nil {
		t.Fatal(err)
	}
	var failed, invalid, accepted int
	for _, line := range splitLines(string(raw)) {
		ev := ParseSSHLine(line)
		if ev == nil {
			continue
		}
		switch ev.Type {
		case "ssh_failed":
			failed++
			if ev.SrcIP == "" {
				t.Fatalf("failed event missing ip: %s", line)
			}
		case "ssh_invalid":
			invalid++
		case "ssh_accepted":
			accepted++
		}
	}
	if failed < 7 {
		t.Fatalf("failed=%d want >=7", failed)
	}
	if invalid < 1 {
		t.Fatalf("invalid=%d", invalid)
	}
	if accepted != 2 {
		t.Fatalf("accepted=%d want 2", accepted)
	}
}

func TestParseNginxLog(t *testing.T) {
	raw, err := os.ReadFile(testdata("nginx.access.log"))
	if err != nil {
		t.Fatal(err)
	}
	var probe, failLogin, skip int
	for _, line := range splitLines(string(raw)) {
		ev := ParseNginxLine(line)
		if ev == nil {
			skip++
			continue
		}
		switch ev.Type {
		case "web_probe":
			probe++
		case "web_failed":
			failLogin++
		}
	}
	if probe < 4 {
		t.Fatalf("probe=%d", probe)
	}
	if failLogin != 1 {
		t.Fatalf("web_failed=%d", failLogin)
	}
	if skip < 1 {
		t.Fatalf("expected at least one ignored 200")
	}
}

func TestParseFail2ban(t *testing.T) {
	raw, err := os.ReadFile(testdata("fail2ban.log"))
	if err != nil {
		t.Fatal(err)
	}
	var ban, unban int
	for _, line := range splitLines(string(raw)) {
		ev := ParseFail2banLine(line)
		if ev == nil {
			continue
		}
		if ev.SrcIP != "203.0.113.10" {
			t.Fatalf("ip %s", ev.SrcIP)
		}
		switch ev.Type {
		case "fail2ban_ban":
			ban++
		case "fail2ban_unban":
			unban++
		}
	}
	if ban != 1 || unban != 1 {
		t.Fatalf("ban=%d unban=%d", ban, unban)
	}
}

func TestParseModernSSH(t *testing.T) {
	lines := []string{
		`2026-09-13T10:59:34.805625+08:00 4C8G sshd-session[482673]: Failed password for root from 193.47.62.69 port 34018 ssh2`,
		`2026-09-13T10:59:33.056774+08:00 4C8G sshd-session[482673]: pam_unix(sshd:auth): authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=193.47.62.69  user=root`,
		`2026-09-13T11:02:45.121389+08:00 4C8G sshd-session[484839]: Invalid user admin from 88.249.10.161 port 50011`,
		`2026-09-13T10:47:02.327757+08:00 4C8G sshd[3340587]: Timeout before authentication for connection from 62.60.130.242 to 64.44.157.217, pid = 478619`,
		`2026-09-13T11:03:09.722230+08:00 4C8G sshd-session[484980]: Connection closed by authenticating user root 195.178.110.228 port 38804 [preauth]`,
	}
	var failed, invalid, timeout, skipped int
	for _, line := range lines {
		ev := ParseSSHLine(line)
		if ev == nil {
			skipped++
			continue
		}
		if ev.Ts < 1700000000 {
			t.Fatalf("bad ts %d for %s", ev.Ts, line)
		}
		switch ev.Type {
		case "ssh_failed":
			failed++
		case "ssh_invalid":
			invalid++
		case "ssh_timeout":
			timeout++
		}
	}
	if failed != 1 || invalid != 1 || timeout != 1 || skipped != 2 {
		t.Fatalf("failed=%d invalid=%d timeout=%d skipped=%d", failed, invalid, timeout, skipped)
	}
}

func TestNginxIgnoresBoringRoot(t *testing.T) {
	line := `16.5.0.236 - - [13/Sep/2026:00:57:26 +0800] "GET / HTTP/1.1" 301 178 "-" "python-requests/2.32.0"`
	if ev := ParseNginxLine(line); ev != nil {
		t.Fatalf("root should be ignored: %+v", ev)
	}
	probe := `203.0.113.88 - - [13/Sep/2026:10:16:01 +0000] "GET /.env HTTP/1.1" 404 162 "-" "python-requests/2.32.0"`
	ev := ParseNginxLine(probe)
	if ev == nil || ev.Type != "web_probe" {
		t.Fatalf("expected probe %+v", ev)
	}
}

func TestParseFail2banTime(t *testing.T) {
	ev := ParseFail2banLine(`2026-09-13 10:17:01,123 fail2ban.actions [12]: NOTICE  [sshd] Ban 203.0.113.10`)
	if ev == nil {
		t.Fatal("nil")
	}
	want, err := time.ParseInLocation("2006-01-02 15:04:05", "2026-09-13 10:17:01", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Ts < want.Unix()-2 || ev.Ts > want.Unix()+2 {
		t.Fatalf("ts %d want ~%d", ev.Ts, want.Unix())
	}
}

func TestParseNftSets(t *testing.T) {
	raw := []byte(`
table inet filter {
	set blacklist {
		type ipv4_addr
		elements = { 1.2.3.4, 5.6.7.8 }
	}
	set allow {
		type ipv4_addr
		elements = { 9.9.9.9 }
	}
}
`)
	got := parseNftSets(raw)
	if got["1.2.3.4"] == "" || got["5.6.7.8"] == "" {
		t.Fatalf("%v", got)
	}
	if _, ok := got["9.9.9.9"]; ok {
		t.Fatalf("allow set should be ignored: %v", got)
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
