package store

import (
	"path/filepath"
	"testing"
)

func TestSetAlertStatusMany(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.RaiseAlert("ssh.bruteforce", "203.0.113.1", "high", "a"); err != nil {
		t.Fatal(err)
	}
	if err := st.RaiseAlert("web.probe", "203.0.113.2", "medium", "b"); err != nil {
		t.Fatal(err)
	}
	open, err := st.OpenAlerts("")
	if err != nil || len(open) != 2 {
		t.Fatalf("open=%d err=%v", len(open), err)
	}
	ids := []int64{open[0].ID, open[1].ID}
	n, err := st.SetAlertStatusMany(ids, "acked")
	if err != nil || n != 2 {
		t.Fatalf("acked n=%d err=%v", n, err)
	}
	n, err = st.SetAlertStatusMany(ids, "acked")
	if err != nil || n != 0 {
		t.Fatalf("already acked n=%d err=%v", n, err)
	}
	n, err = st.SetAlertStatusMany(ids, "resolved")
	if err != nil || n != 2 {
		t.Fatalf("resolved n=%d err=%v", n, err)
	}
	left, err := st.OpenAlerts("")
	if err != nil || len(left) != 0 {
		t.Fatalf("left=%+v err=%v", left, err)
	}
}
