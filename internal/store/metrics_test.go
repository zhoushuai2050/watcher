package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueryMetricsAcceptsAveragedFloats(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.db.Exec(`INSERT INTO metric_samples(ts,step,cpu,mem_used,mem_total,swap_used,swap_total,load1,load5,load15,disk_used_pct,net_in_bps,net_out_bps,disk_read_bps,disk_write_bps,temp_c)
		VALUES(1000,60,1.5,100.2,200.8,0,0,0.1,0.2,0.3,10.5,12345.5,89181.25,10.2,20.8,41.2)`)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.QueryMetrics(0, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].NetOutBps != 89181 {
		t.Fatalf("net_out=%d", rows[0].NetOutBps)
	}
	if rows[0].MemTotal != 201 {
		t.Fatalf("mem_total=%d", rows[0].MemTotal)
	}
}

func TestDownsampleThenQuery(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := int64(1_700_000_000)
	for i := int64(0); i < 12; i++ {
		if err := st.InsertMetric(Metric{
			Ts: base + i*5, Step: 5, CPU: 10, MemUsed: 100, MemTotal: 200,
			NetInBps: 1000 + i, NetOutBps: 2000 + i, DiskReadBps: 3, DiskWriteBps: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.db.Exec(`INSERT INTO metric_samples(ts,step,cpu,mem_used,mem_total,net_in_bps,net_out_bps,disk_read_bps,disk_write_bps)
		SELECT (ts/60)*60, 60, avg(cpu), avg(mem_used), avg(mem_total), avg(net_in_bps), avg(net_out_bps), avg(disk_read_bps), avg(disk_write_bps)
		FROM metric_samples WHERE step=5 GROUP BY (ts/60)*60`); err != nil {
		t.Fatal(err)
	}
	rows, err := st.QueryMetrics(base-10, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no downsampled rows")
	}
	if rows[0].NetOutBps <= 0 {
		t.Fatalf("net_out=%d", rows[0].NetOutBps)
	}
}

func TestQueryLiveRanges(t *testing.T) {
	path := os.Getenv("WATCHER_DB")
	if path == "" {
		t.Skip("WATCHER_DB not set")
	}
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().Unix()
	for _, spec := range []struct {
		name string
		from int64
		step int
	}{
		{"1h", now - 3600, 5},
		{"6h", now - 6*3600, 60},
		{"24h", now - 86400, 60},
		{"7d", now - 7*86400, 300},
	} {
		rows, err := st.QueryMetrics(spec.from, spec.step)
		if err != nil {
			t.Fatalf("%s: %v", spec.name, err)
		}
		if len(rows) == 0 {
			t.Fatalf("%s: no rows", spec.name)
		}
	}
}
