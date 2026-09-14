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
