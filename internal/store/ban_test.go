package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAttackFailAggAndBans(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().Unix()
	for i := 0; i < 3; i++ {
		if _, err := st.InsertEvent(Event{Ts: now, Source: "ssh", Type: "ssh_failed", SrcIP: "203.0.113.9"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.InsertEvent(Event{Ts: now, Source: "ssh", Type: "ssh_accepted", SrcIP: "203.0.113.9"}); err != nil {
		t.Fatal(err)
	}
	n, err := st.CountAttackFailsIP("203.0.113.9", now-60)
	if err != nil || n != 3 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	created, err := st.InsertBan(IPBan{IP: "203.0.113.9", Reason: "t", Hits: 3, Auto: true})
	if err != nil || !created {
		t.Fatalf("insert created=%v err=%v", created, err)
	}
	again, err := st.InsertBan(IPBan{IP: "203.0.113.9", Reason: "t2", Hits: 4, Auto: true})
	if err != nil || again {
		t.Fatalf("dup created=%v err=%v", again, err)
	}
	rows, err := st.QueryIPAgg(now - 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Banned || rows[0].Fails != 3 {
		t.Fatalf("%+v", rows)
	}
}

func TestBansPageFuzzy(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for i, ip := range []string{"203.0.113.1", "203.0.113.2", "198.51.100.7"} {
		if _, err := st.InsertBan(IPBan{IP: ip, Reason: "scan", Hits: i + 1, Auto: true}); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := st.ListBansPage("203.0.113", 1, 2)
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("items=%d total=%d err=%v", len(items), total, err)
	}
	items, total, err = st.ListBansPage("198.51", 1, 20)
	if err != nil || total != 1 || items[0].IP != "198.51.100.7" {
		t.Fatalf("%+v total=%d err=%v", items, total, err)
	}
}

func TestAllowlist(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	spec, ok := NormalizeAllow("198.51.100.10/24")
	if !ok || spec != "198.51.100.0/24" {
		t.Fatalf("spec=%s ok=%v", spec, ok)
	}
	if err := st.InsertAllow(IPAllow{Spec: spec, Note: "lab"}); err != nil {
		t.Fatal(err)
	}
	if !AllowContains(spec, "198.51.100.20") || AllowContains(spec, "203.0.113.1") {
		t.Fatal("cidr match")
	}
	yes, err := st.IsAllowed("198.51.100.20")
	if err != nil || !yes {
		t.Fatalf("allowed=%v err=%v", yes, err)
	}
	list, err := st.ListAllow()
	if err != nil || len(list) != 1 || list[0].Note != "lab" {
		t.Fatalf("%+v %v", list, err)
	}
	if err := st.DeleteAllow(spec); err != nil {
		t.Fatal(err)
	}
	yes, _ = st.IsAllowed("198.51.100.20")
	if yes {
		t.Fatal("deleted")
	}
}
