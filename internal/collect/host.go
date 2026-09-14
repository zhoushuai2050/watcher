package collect

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/sensors"

	"watcher/internal/store"
)

type ioSnap struct {
	ts  time.Time
	in  uint64
	out uint64
	r   uint64
	w   uint64
}

func (e *Engine) collectHost() error {
	now := time.Now()
	sample := HostSample{}
	if info, err := host.Info(); err == nil {
		sample.Hostname = info.Hostname
		sample.OS = strings.TrimSpace(info.Platform + " " + info.PlatformVersion)
		sample.UptimeSec = info.Uptime
	}
	if n, err := cpu.Counts(true); err == nil {
		sample.CPUCores = n
	}
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		sample.CPUPct = pct[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		sample.MemUsed = int64(vm.Used)
		sample.MemTotal = int64(vm.Total)
	}
	if sm, err := mem.SwapMemory(); err == nil {
		sample.SwapUsed = int64(sm.Used)
		sample.SwapTotal = int64(sm.Total)
	}
	if avg, err := load.Avg(); err == nil {
		sample.Load1, sample.Load5, sample.Load15 = avg.Load1, avg.Load5, avg.Load15
	}
	if temps, err := sensors.SensorsTemperatures(); err == nil {
		for _, t := range temps {
			name := strings.ToLower(t.SensorKey)
			if t.Temperature <= 0 {
				continue
			}
			if strings.Contains(name, "cpu") || strings.Contains(name, "core") || strings.Contains(name, "pkg") || sample.TempC == nil {
				v := t.Temperature
				sample.TempC = &v
				if strings.Contains(name, "cpu") || strings.Contains(name, "pkg") {
					break
				}
			}
		}
	}

	skipFS := map[string]bool{"tmpfs": true, "devtmpfs": true, "overlay": true, "squashfs": true, "proc": true, "sysfs": true, "cgroup2": true, "cgroup": true}
	if parts, err := disk.Partitions(false); err == nil {
		var maxPct float64
		for _, p := range parts {
			if skipFS[p.Fstype] || strings.HasPrefix(p.Mountpoint, "/snap") {
				continue
			}
			u, err := disk.Usage(p.Mountpoint)
			if err != nil || u.Total == 0 {
				continue
			}
			d := DiskInfo{Mount: p.Mountpoint, Fstype: p.Fstype, Used: int64(u.Used), Total: int64(u.Total), Pct: u.UsedPercent}
			sample.Disks = append(sample.Disks, d)
			if u.UsedPercent > maxPct && (p.Mountpoint == "/" || maxPct == 0) {
				maxPct = u.UsedPercent
			}
			if p.Mountpoint == "/" {
				maxPct = u.UsedPercent
			}
		}
		sample.DiskUsedPct = maxPct
		if sample.DiskUsedPct == 0 && len(sample.Disks) > 0 {
			sample.DiskUsedPct = sample.Disks[0].Pct
		}
	}

	var in, out, rB, wB uint64
	if ios, err := net.IOCounters(true); err == nil {
		for _, nic := range ios {
			if nic.Name == "lo" || strings.HasPrefix(nic.Name, "veth") || strings.HasPrefix(nic.Name, "br-") || strings.HasPrefix(nic.Name, "docker") {
				continue
			}
			in += nic.BytesRecv
			out += nic.BytesSent
		}
	}
	if dios, err := disk.IOCounters(); err == nil {
		for _, d := range dios {
			rB += d.ReadBytes
			wB += d.WriteBytes
		}
	}
	if !e.netPrev.ts.IsZero() {
		dt := now.Sub(e.netPrev.ts).Seconds()
		if dt > 0.2 {
			sample.NetInBps = int64(float64(in-e.netPrev.in) / dt)
			sample.NetOutBps = int64(float64(out-e.netPrev.out) / dt)
			sample.DiskReadBps = int64(float64(rB-e.netPrev.r) / dt)
			sample.DiskWriteBps = int64(float64(wB-e.netPrev.w) / dt)
			if sample.NetInBps < 0 {
				sample.NetInBps = 0
			}
			if sample.NetOutBps < 0 {
				sample.NetOutBps = 0
			}
		}
	}
	e.netPrev = ioSnap{ts: now, in: in, out: out, r: rB, w: wB}

	extra, _ := json.Marshal(map[string]any{"disks": sample.Disks})
	m := store.Metric{
		Ts: now.Unix(), Step: 5, CPU: sample.CPUPct,
		MemUsed: sample.MemUsed, MemTotal: sample.MemTotal,
		SwapUsed: sample.SwapUsed, SwapTotal: sample.SwapTotal,
		Load1: sample.Load1, Load5: sample.Load5, Load15: sample.Load15,
		DiskUsedPct: sample.DiskUsedPct, NetInBps: sample.NetInBps, NetOutBps: sample.NetOutBps,
		DiskReadBps: sample.DiskReadBps, DiskWriteBps: sample.DiskWriteBps,
		TempC: sample.TempC, Extra: string(extra),
	}
	if err := e.store.InsertMetric(m); err != nil {
		return err
	}
	e.mu.Lock()
	e.host = sample
	e.mu.Unlock()
	return nil
}
