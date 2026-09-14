package collect

import "watcher/internal/store"

type Status struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	OK        bool   `json:"ok"`
	LastRun   int64  `json:"last_run"`
	LastError string `json:"last_error"`
	Detail    string `json:"detail"`
}

type DiskInfo struct {
	Mount  string  `json:"mount"`
	Fstype string  `json:"fstype"`
	Used   int64   `json:"used"`
	Total  int64   `json:"total"`
	Pct    float64 `json:"pct"`
}

type HostSample struct {
	Hostname     string     `json:"hostname"`
	OS           string     `json:"os"`
	UptimeSec    uint64     `json:"uptime_sec"`
	CPUPct       float64    `json:"cpu_pct"`
	CPUCores     int        `json:"cpu_cores"`
	MemUsed      int64      `json:"mem_used"`
	MemTotal     int64      `json:"mem_total"`
	SwapUsed     int64      `json:"swap_used"`
	SwapTotal    int64      `json:"swap_total"`
	Load1        float64    `json:"load1"`
	Load5        float64    `json:"load5"`
	Load15       float64    `json:"load15"`
	DiskUsedPct  float64    `json:"disk_used_pct"`
	NetInBps     int64      `json:"net_in_bps"`
	NetOutBps    int64      `json:"net_out_bps"`
	DiskReadBps  int64      `json:"disk_read_bps"`
	DiskWriteBps int64      `json:"disk_write_bps"`
	TempC        *float64   `json:"temp_c"`
	Disks        []DiskInfo `json:"disks"`
}

type ConnRow struct {
	SrcIP   string `json:"src_ip"`
	DstPort int    `json:"dst_port"`
	State   string `json:"state"`
	Count   int    `json:"count"`
}

type NetSnapshot struct {
	Ts     int64              `json:"ts"`
	Listen []store.ListenPort `json:"listen"`
	Conns  []ConnRow          `json:"conns"`
	States map[string]int     `json:"states"`
}

type ProcGroup struct {
	Key   string  `json:"key"`
	Kind  string  `json:"kind"`
	CPU   float64 `json:"cpu"`
	RSS   int64   `json:"rss"`
	Count int     `json:"count"`
}
