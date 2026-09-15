package collect

import (
	"path/filepath"
	"strings"
	"testing"

	"watcher/internal/config"
	"watcher/internal/store"
)

func TestBanPermanentAndUnban(t *testing.T) {
	var calls []string
	defer SetRunNft(func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	})()

	st, err := store.Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, config.Defaults())
	if err := eng.BanPermanent("203.0.113.50", "test", false); err != nil {
		t.Fatal(err)
	}
	ok, err := st.IsBanned("203.0.113.50")
	if err != nil || !ok {
		t.Fatalf("banned=%v err=%v", ok, err)
	}
	if err := eng.BanPermanent("127.0.0.1", "x", false); err == nil {
		t.Fatal("loopback should fail")
	}
	if err := eng.BanPermanent("10.0.0.8", "auto", true); err != nil {
		t.Fatal(err)
	}
	priv, _ := st.IsBanned("10.0.0.8")
	if priv {
		t.Fatal("auto should skip private")
	}
	if err := eng.UnbanPermanent("203.0.113.50"); err != nil {
		t.Fatal(err)
	}
	ok, _ = st.IsBanned("203.0.113.50")
	if ok {
		t.Fatal("still banned")
	}
	if err := eng.AddAllow("203.0.113.80", "office"); err != nil {
		t.Fatal(err)
	}
	if err := eng.BanPermanent("203.0.113.80", "nope", false); err == nil {
		t.Fatal("whitelist should block manual ban")
	}
	if err := eng.BanPermanent("203.0.113.80", "nope", true); err != nil {
		t.Fatal(err)
	}
	banned, _ := st.IsBanned("203.0.113.80")
	if banned {
		t.Fatal("auto should skip whitelist")
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "add element inet watcher permanent4") {
		t.Fatalf("calls=%v", calls)
	}
}
