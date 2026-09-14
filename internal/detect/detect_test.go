package detect

import (
	"path/filepath"
	"testing"
	"time"

	"watcher/internal/collect"
	"watcher/internal/config"
	"watcher/internal/store"
)

func TestSSHBruteforceAlert(t *testing.T) {
	defer collect.SetRunNft(func(args ...string) (string, error) { return "", nil })()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	eng := collect.New(st, cfg)
	d := New(st, cfg, eng)
	now := time.Now().Unix()
	for i := 0; i < 8; i++ {
		if _, err := st.InsertEvent(store.Event{
			Ts: now - 10, Source: "ssh", Type: "ssh_failed", Severity: "medium",
			SrcIP: "203.0.113.10", User: "root", Message: "fail",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.tickOnce(); err != nil {
		t.Fatal(err)
	}
	alerts, err := st.OpenAlerts("ssh.bruteforce")
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Key != "203.0.113.10" {
		t.Fatalf("%+v", alerts)
	}
}

func TestPermanentBanAtThreshold(t *testing.T) {
	defer collect.SetRunNft(func(args ...string) (string, error) { return "", nil })()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	cfg.Settings.AttackBanThreshold = 3
	eng := collect.New(st, cfg)
	d := New(st, cfg, eng)
	now := time.Now().Unix()
	for i := 0; i < 2; i++ {
		if _, err := st.InsertEvent(store.Event{Ts: now, Source: "ssh", Type: "ssh_failed", SrcIP: "203.0.113.77"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.tickOnce(); err != nil {
		t.Fatal(err)
	}
	banned, _ := st.IsBanned("203.0.113.77")
	if banned {
		t.Fatal("should not ban below threshold")
	}
	if _, err := st.InsertEvent(store.Event{Ts: now, Source: "ssh", Type: "ssh_failed", SrcIP: "203.0.113.77"}); err != nil {
		t.Fatal(err)
	}
	if err := d.tickOnce(); err != nil {
		t.Fatal(err)
	}
	banned, _ = st.IsBanned("203.0.113.77")
	if !banned {
		t.Fatal("want permanent ban at threshold 3")
	}
}

func TestSuccessAfterFail(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	d := New(st, cfg, collect.New(st, cfg))
	now := time.Now().Unix()
	for i := 0; i < 4; i++ {
		_, _ = st.InsertEvent(store.Event{Ts: now - 30, Source: "ssh", Type: "ssh_failed", SrcIP: "1.2.3.4", User: "root"})
	}
	d.OnEvent(store.Event{Ts: now, Source: "ssh", Type: "ssh_accepted", SrcIP: "1.2.3.4", User: "root"})
	alerts, _ := st.OpenAlerts("ssh.success_after_fail")
	if len(alerts) != 1 {
		t.Fatalf("want success_after_fail, got %+v", alerts)
	}
	newSrc, _ := st.OpenAlerts("ssh.new_source")
	if len(newSrc) != 1 {
		t.Fatalf("want new_source %+v", newSrc)
	}
}
