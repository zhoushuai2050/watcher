package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Listen        string   `toml:"listen"`
	DataDir       string   `toml:"data_dir"`
	PublicPath    string   `toml:"public_path"`
	AdminUser     string   `toml:"admin_user"`
	AdminPassword string   `toml:"admin_password"`
	JWTSecret     string   `toml:"jwt_secret"`
	JWTDays       int      `toml:"jwt_days"`
	AuthLog       string   `toml:"auth_log"`
	NginxLog      string   `toml:"nginx_log"`
	Fail2banLog   string   `toml:"fail2ban_log"`
	KernLog       string   `toml:"kern_log"`
	ListenAllow   []string `toml:"listen_allow"`
	Settings      Settings `toml:"detect"`
}

type Settings struct {
	CPUPct             float64  `toml:"cpu_pct" json:"cpu_pct"`
	MemPct             float64  `toml:"mem_pct" json:"mem_pct"`
	DiskPct            float64  `toml:"disk_pct" json:"disk_pct"`
	CPUSustainSec      int      `toml:"cpu_sustain_sec" json:"cpu_sustain_sec"`
	SSHFailCount       int      `toml:"ssh_fail_count" json:"ssh_fail_count"`
	SSHFailWindowSec   int      `toml:"ssh_fail_window_sec" json:"ssh_fail_window_sec"`
	InvalidUserCount   int      `toml:"invalid_user_count" json:"invalid_user_count"`
	WebProbeCount      int      `toml:"web_probe_count" json:"web_probe_count"`
	WebProbeWindowSec  int      `toml:"web_probe_window_sec" json:"web_probe_window_sec"`
	WebFailCount       int      `toml:"web_fail_count" json:"web_fail_count"`
	PortScanPorts      int      `toml:"port_scan_ports" json:"port_scan_ports"`
	PortScanWindowSec  int      `toml:"port_scan_window_sec" json:"port_scan_window_sec"`
	ConnFloodCount     int      `toml:"conn_flood_count" json:"conn_flood_count"`
	SynRecvCount       int      `toml:"syn_recv_count" json:"syn_recv_count"`
	ProcCPUPct         float64  `toml:"proc_cpu_pct" json:"proc_cpu_pct"`
	ListenAllow        []string `toml:"listen_allow" json:"listen_allow"`
	AuthLog            string   `toml:"auth_log" json:"auth_log"`
	NginxLog           string   `toml:"nginx_log" json:"nginx_log"`
	Fail2banLog        string   `toml:"fail2ban_log" json:"fail2ban_log"`
	AttackBanThreshold int      `toml:"attack_ban_threshold" json:"attack_ban_threshold"`
}

func Defaults() Config {
	return Config{
		Listen:        "127.0.0.1:8020",
		DataDir:       "./data",
		PublicPath:    "/Watcher",
		AdminUser:     "admin",
		AdminPassword: "watcher",
		JWTSecret:     "change-me-watcher-secret",
		JWTDays:       7,
		NginxLog:      "/var/log/nginx/access.log",
		Fail2banLog:   "/var/log/fail2ban.log",
		KernLog:       "/var/log/kern.log",
		ListenAllow:   []string{"tcp:22", "tcp:80", "tcp:443", "tcp:53", "udp:53", "tcp:8020", "tcp:8010", "tcp:8011"},
		Settings: Settings{
			CPUPct:             90,
			MemPct:             90,
			DiskPct:            90,
			CPUSustainSec:      120,
			SSHFailCount:       5,
			SSHFailWindowSec:   600,
			InvalidUserCount:   5,
			WebProbeCount:      10,
			WebProbeWindowSec:  600,
			WebFailCount:       12,
			PortScanPorts:      12,
			PortScanWindowSec:  60,
			ConnFloodCount:     80,
			SynRecvCount:       64,
			ProcCPUPct:         80,
			AttackBanThreshold: 3,
		},
	}
}

func Load() Config {
	cfg := Defaults()
	path := ""
	listen := ""
	dataDir := ""
	flag.StringVar(&path, "config", "", "config toml path")
	flag.StringVar(&listen, "listen", "", "listen address")
	flag.StringVar(&dataDir, "data", "", "data directory")
	flag.Parse()

	if path == "" {
		path = firstExisting("watcher.toml", "/etc/watcher/config.toml")
	}
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read config %s: %v\n", path, err)
			os.Exit(2)
		}
		if err := toml.Unmarshal(raw, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "parse config %s: %v\n", path, err)
			os.Exit(2)
		}
	}
	if listen != "" {
		cfg.Listen = listen
	}
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	if v := os.Getenv("WATCHER_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("WATCHER_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("WATCHER_ADMIN_USER"); v != "" {
		cfg.AdminUser = v
	}
	if v := os.Getenv("WATCHER_ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("WATCHER_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if cfg.Settings.ListenAllow == nil {
		cfg.Settings.ListenAllow = append([]string{}, cfg.ListenAllow...)
	}
	if cfg.Settings.AuthLog == "" {
		cfg.Settings.AuthLog = cfg.AuthLog
	}
	if cfg.Settings.NginxLog == "" {
		cfg.Settings.NginxLog = cfg.NginxLog
	}
	if cfg.Settings.Fail2banLog == "" {
		cfg.Settings.Fail2banLog = cfg.Fail2banLog
	}
	return cfg
}

func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "watcher.db")
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func SplitListen(spec string) (proto, port string) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if spec == "" {
		return "", ""
	}
	if i := strings.IndexByte(spec, ':'); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return "tcp", spec
}
