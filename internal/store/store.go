package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"watcher/internal/config"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS metric_samples (
  ts INTEGER NOT NULL,
  step INTEGER NOT NULL,
  cpu REAL,
  mem_used INTEGER,
  mem_total INTEGER,
  swap_used INTEGER,
  swap_total INTEGER,
  load1 REAL,
  load5 REAL,
  load15 REAL,
  disk_used_pct REAL,
  net_in_bps INTEGER,
  net_out_bps INTEGER,
  disk_read_bps INTEGER,
  disk_write_bps INTEGER,
  temp_c REAL,
  extra TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_metric_unique ON metric_samples(step, ts);
CREATE TABLE IF NOT EXISTS process_snapshots (
  ts INTEGER NOT NULL,
  pid INTEGER,
  name TEXT,
  unit TEXT,
  container TEXT,
  user_name TEXT,
  cpu REAL,
  rss INTEGER,
  read_bytes INTEGER,
  write_bytes INTEGER,
  cmdline TEXT
);
CREATE INDEX IF NOT EXISTS idx_proc_ts ON process_snapshots(ts);
CREATE TABLE IF NOT EXISTS listen_ports (
  proto TEXT NOT NULL,
  addr TEXT NOT NULL,
  port INTEGER NOT NULL,
  pid INTEGER,
  name TEXT,
  first_seen INTEGER NOT NULL,
  last_seen INTEGER NOT NULL,
  PRIMARY KEY (proto, addr, port)
);
CREATE TABLE IF NOT EXISTS security_events (
  id INTEGER PRIMARY KEY,
  ts INTEGER NOT NULL,
  source TEXT NOT NULL,
  type TEXT NOT NULL,
  severity TEXT NOT NULL,
  src_ip TEXT,
  user_name TEXT,
  path TEXT,
  message TEXT,
  raw TEXT
);
CREATE INDEX IF NOT EXISTS idx_sec_ts ON security_events(ts);
CREATE INDEX IF NOT EXISTS idx_sec_ip ON security_events(src_ip);
CREATE INDEX IF NOT EXISTS idx_sec_src ON security_events(source, type, ts);
CREATE TABLE IF NOT EXISTS alerts (
  id INTEGER PRIMARY KEY,
  rule_id TEXT NOT NULL,
  key TEXT NOT NULL,
  severity TEXT NOT NULL,
  status TEXT NOT NULL,
  first_seen INTEGER NOT NULL,
  last_seen INTEGER NOT NULL,
  count INTEGER NOT NULL,
  summary TEXT
);
CREATE INDEX IF NOT EXISTS idx_alert_open ON alerts(rule_id, key, status);
CREATE TABLE IF NOT EXISTS known_ssh_ips (
  ip TEXT PRIMARY KEY,
  first_seen INTEGER,
  last_seen INTEGER,
  user_name TEXT
);
CREATE TABLE IF NOT EXISTS ip_bans (
  ip TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  reason TEXT,
  hits INTEGER NOT NULL DEFAULT 0,
  auto INTEGER NOT NULL DEFAULT 1
);
`)
	return err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) GetMeta(key string) (string, bool) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	if err != nil {
		return "", false
	}
	return v, true
}

type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
}

func (s *Store) UserCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) InsertUser(name, hash string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO users(name, password_hash, created_at) VALUES(?,?,?)`, name, hash, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) GetUserByName(name string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(`SELECT id, name, password_hash FROM users WHERE name=?`, name).Scan(&u.ID, &u.Name, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(`SELECT id, name, password_hash FROM users WHERE id=?`, id).Scan(&u.ID, &u.Name, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) UpdatePassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, id)
	return err
}

type Metric struct {
	Ts           int64    `json:"ts"`
	Step         int      `json:"step"`
	CPU          float64  `json:"cpu"`
	MemUsed      int64    `json:"mem_used"`
	MemTotal     int64    `json:"mem_total"`
	SwapUsed     int64    `json:"swap_used"`
	SwapTotal    int64    `json:"swap_total"`
	Load1        float64  `json:"load1"`
	Load5        float64  `json:"load5"`
	Load15       float64  `json:"load15"`
	DiskUsedPct  float64  `json:"disk_used_pct"`
	NetInBps     int64    `json:"net_in_bps"`
	NetOutBps    int64    `json:"net_out_bps"`
	DiskReadBps  int64    `json:"disk_read_bps"`
	DiskWriteBps int64    `json:"disk_write_bps"`
	TempC        *float64 `json:"temp_c"`
	Extra        string   `json:"extra,omitempty"`
}

func (s *Store) InsertMetric(m Metric) error {
	if m.Step == 0 {
		m.Step = 5
	}
	_, err := s.db.Exec(`INSERT INTO metric_samples(
		ts,step,cpu,mem_used,mem_total,swap_used,swap_total,load1,load5,load15,
		disk_used_pct,net_in_bps,net_out_bps,disk_read_bps,disk_write_bps,temp_c,extra)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(step,ts) DO UPDATE SET
		cpu=excluded.cpu, mem_used=excluded.mem_used, mem_total=excluded.mem_total,
		swap_used=excluded.swap_used, swap_total=excluded.swap_total,
		load1=excluded.load1, load5=excluded.load5, load15=excluded.load15,
		disk_used_pct=excluded.disk_used_pct, net_in_bps=excluded.net_in_bps,
		net_out_bps=excluded.net_out_bps, disk_read_bps=excluded.disk_read_bps,
		disk_write_bps=excluded.disk_write_bps, temp_c=excluded.temp_c, extra=excluded.extra`,
		m.Ts, m.Step, m.CPU, m.MemUsed, m.MemTotal, m.SwapUsed, m.SwapTotal, m.Load1, m.Load5, m.Load15,
		m.DiskUsedPct, m.NetInBps, m.NetOutBps, m.DiskReadBps, m.DiskWriteBps, m.TempC, m.Extra)
	return err
}

func asInt64(v float64) int64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v))
}

func (s *Store) QueryMetrics(from int64, step int) ([]Metric, error) {
	rows, err := s.db.Query(`SELECT ts,step,COALESCE(cpu,0),
		COALESCE(mem_used,0), COALESCE(mem_total,0), COALESCE(swap_used,0), COALESCE(swap_total,0),
		COALESCE(load1,0), COALESCE(load5,0), COALESCE(load15,0), COALESCE(disk_used_pct,0),
		COALESCE(net_in_bps,0), COALESCE(net_out_bps,0), COALESCE(disk_read_bps,0), COALESCE(disk_write_bps,0),
		temp_c, extra
		FROM metric_samples WHERE step=? AND ts>=? ORDER BY ts`, step, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Metric, 0, 256)
	for rows.Next() {
		var m Metric
		var memUsed, memTotal, swapUsed, swapTotal, netIn, netOut, diskR, diskW float64
		var extra sql.NullString
		if err := rows.Scan(&m.Ts, &m.Step, &m.CPU, &memUsed, &memTotal, &swapUsed, &swapTotal,
			&m.Load1, &m.Load5, &m.Load15, &m.DiskUsedPct, &netIn, &netOut, &diskR, &diskW, &m.TempC, &extra); err != nil {
			return nil, err
		}
		m.MemUsed = asInt64(memUsed)
		m.MemTotal = asInt64(memTotal)
		m.SwapUsed = asInt64(swapUsed)
		m.SwapTotal = asInt64(swapTotal)
		m.NetInBps = asInt64(netIn)
		m.NetOutBps = asInt64(netOut)
		m.DiskReadBps = asInt64(diskR)
		m.DiskWriteBps = asInt64(diskW)
		if extra.Valid {
			m.Extra = extra.String
		}
		out = append(out, m)
	}
	if len(out) == 0 && step != 5 {
		return s.QueryMetrics(from, 5)
	}
	return out, rows.Err()
}

func (s *Store) Downsample() error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(`INSERT INTO metric_samples(ts,step,cpu,mem_used,mem_total,swap_used,swap_total,load1,load5,load15,disk_used_pct,net_in_bps,net_out_bps,disk_read_bps,disk_write_bps,temp_c)
		SELECT (ts/60)*60, 60, avg(cpu),
		CAST(ROUND(avg(mem_used)) AS INTEGER), CAST(ROUND(avg(mem_total)) AS INTEGER),
		CAST(ROUND(avg(swap_used)) AS INTEGER), CAST(ROUND(avg(swap_total)) AS INTEGER),
		avg(load1), avg(load5), avg(load15), avg(disk_used_pct),
		CAST(ROUND(avg(net_in_bps)) AS INTEGER), CAST(ROUND(avg(net_out_bps)) AS INTEGER),
		CAST(ROUND(avg(disk_read_bps)) AS INTEGER), CAST(ROUND(avg(disk_write_bps)) AS INTEGER),
		avg(temp_c)
		FROM metric_samples WHERE step=5 AND ts>=? GROUP BY (ts/60)*60
		ON CONFLICT(step,ts) DO NOTHING`, now-3600); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO metric_samples(ts,step,cpu,mem_used,mem_total,swap_used,swap_total,load1,load5,load15,disk_used_pct,net_in_bps,net_out_bps,disk_read_bps,disk_write_bps,temp_c)
		SELECT (ts/300)*300, 300, avg(cpu),
		CAST(ROUND(avg(mem_used)) AS INTEGER), CAST(ROUND(avg(mem_total)) AS INTEGER),
		CAST(ROUND(avg(swap_used)) AS INTEGER), CAST(ROUND(avg(swap_total)) AS INTEGER),
		avg(load1), avg(load5), avg(load15), avg(disk_used_pct),
		CAST(ROUND(avg(net_in_bps)) AS INTEGER), CAST(ROUND(avg(net_out_bps)) AS INTEGER),
		CAST(ROUND(avg(disk_read_bps)) AS INTEGER), CAST(ROUND(avg(disk_write_bps)) AS INTEGER),
		avg(temp_c)
		FROM metric_samples WHERE step IN (5,60) AND ts>=? GROUP BY (ts/300)*300
		ON CONFLICT(step,ts) DO NOTHING`, now-6*3600)
	return err
}

func (s *Store) Prune() error {
	now := time.Now().Unix()
	stmts := []string{
		fmt.Sprintf(`DELETE FROM metric_samples WHERE step=5 AND ts<%d`, now-86400),
		fmt.Sprintf(`DELETE FROM metric_samples WHERE step=60 AND ts<%d`, now-7*86400),
		fmt.Sprintf(`DELETE FROM metric_samples WHERE step=300 AND ts<%d`, now-30*86400),
		fmt.Sprintf(`DELETE FROM process_snapshots WHERE ts<%d`, now-86400),
		fmt.Sprintf(`DELETE FROM security_events WHERE ts<%d`, now-90*86400),
		fmt.Sprintf(`DELETE FROM alerts WHERE status='resolved' AND last_seen<%d`, now-90*86400),
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

type ProcRow struct {
	Ts         int64   `json:"ts"`
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Unit       string  `json:"unit"`
	Container  string  `json:"container"`
	User       string  `json:"user"`
	CPU        float64 `json:"cpu"`
	RSS        int64   `json:"rss"`
	ReadBytes  int64   `json:"read_bytes"`
	WriteBytes int64   `json:"write_bytes"`
	Cmdline    string  `json:"cmdline"`
}

func (s *Store) InsertProcesses(ts int64, rows []ProcRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO process_snapshots(ts,pid,name,unit,container,user_name,cpu,rss,read_bytes,write_bytes,cmdline) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(ts, r.PID, r.Name, r.Unit, r.Container, r.User, r.CPU, r.RSS, r.ReadBytes, r.WriteBytes, r.Cmdline); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LatestProcesses() ([]ProcRow, error) {
	var ts int64
	err := s.db.QueryRow(`SELECT COALESCE(MAX(ts),0) FROM process_snapshots`).Scan(&ts)
	if err != nil || ts == 0 {
		return []ProcRow{}, err
	}
	rows, err := s.db.Query(`SELECT ts,pid,name,unit,container,user_name,cpu,rss,read_bytes,write_bytes,cmdline FROM process_snapshots WHERE ts=? ORDER BY cpu DESC`, ts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProcRow{}
	for rows.Next() {
		var r ProcRow
		if err := rows.Scan(&r.Ts, &r.PID, &r.Name, &r.Unit, &r.Container, &r.User, &r.CPU, &r.RSS, &r.ReadBytes, &r.WriteBytes, &r.Cmdline); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ProcessHistory(name, container string, from int64) ([]ProcRow, error) {
	q := `SELECT ts, pid, name, unit, container, user_name, cpu, rss, read_bytes, write_bytes, cmdline
		FROM process_snapshots WHERE ts>=?`
	args := []any{from}
	switch {
	case container != "":
		q += ` AND container=?`
		args = append(args, container)
	case name != "":
		q += ` AND name=?`
		args = append(args, name)
	default:
		return []ProcRow{}, nil
	}
	q += ` ORDER BY ts, cpu DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProcRow{}
	for rows.Next() {
		var r ProcRow
		if err := rows.Scan(&r.Ts, &r.PID, &r.Name, &r.Unit, &r.Container, &r.User, &r.CPU, &r.RSS, &r.ReadBytes, &r.WriteBytes, &r.Cmdline); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ListenPort struct {
	Proto     string `json:"proto"`
	Family    string `json:"family"`
	Addr      string `json:"addr"`
	Port      int    `json:"port"`
	PID       int32  `json:"pid"`
	Name      string `json:"name"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
	Allowed   bool   `json:"allowed"`
}

func (s *Store) UpsertListen(now int64, ports []ListenPort) ([]ListenPort, error) {
	var born []ListenPort
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, p := range ports {
		var first int64
		err := tx.QueryRow(`SELECT first_seen FROM listen_ports WHERE proto=? AND addr=? AND port=?`, p.Proto, p.Addr, p.Port).Scan(&first)
		if err == sql.ErrNoRows {
			p.FirstSeen = now
			p.LastSeen = now
			if _, err := tx.Exec(`INSERT INTO listen_ports(proto,addr,port,pid,name,first_seen,last_seen) VALUES(?,?,?,?,?,?,?)`,
				p.Proto, p.Addr, p.Port, p.PID, p.Name, now, now); err != nil {
				return nil, err
			}
			born = append(born, p)
			continue
		}
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE listen_ports SET pid=?, name=?, last_seen=? WHERE proto=? AND addr=? AND port=?`,
			p.PID, p.Name, now, p.Proto, p.Addr, p.Port); err != nil {
			return nil, err
		}
	}
	return born, tx.Commit()
}

func (s *Store) ListListen() ([]ListenPort, error) {
	rows, err := s.db.Query(`SELECT proto,addr,port,pid,name,first_seen,last_seen FROM listen_ports ORDER BY port, proto`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListenPort{}
	for rows.Next() {
		var p ListenPort
		if err := rows.Scan(&p.Proto, &p.Addr, &p.Port, &p.PID, &p.Name, &p.FirstSeen, &p.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type Event struct {
	ID       int64  `json:"id"`
	Ts       int64  `json:"ts"`
	Source   string `json:"source"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	SrcIP    string `json:"src_ip"`
	User     string `json:"user"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Raw      string `json:"raw,omitempty"`
}

func (s *Store) InsertEvent(ev Event) (int64, error) {
	if ev.Ts == 0 {
		ev.Ts = time.Now().Unix()
	}
	res, err := s.db.Exec(`INSERT INTO security_events(ts,source,type,severity,src_ip,user_name,path,message,raw) VALUES(?,?,?,?,?,?,?,?,?)`,
		ev.Ts, ev.Source, ev.Type, ev.Severity, ev.SrcIP, ev.User, ev.Path, ev.Message, ev.Raw)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) QueryEvents(source, ip string, from int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id,ts,source,type,severity,src_ip,user_name,path,message FROM security_events WHERE ts>=?`
	args := []any{from}
	if source != "" {
		q += ` AND source=?`
		args = append(args, source)
	}
	if ip != "" {
		q += ` AND src_ip=?`
		args = append(args, ip)
	}
	q += ` ORDER BY ts DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Ts, &e.Source, &e.Type, &e.Severity, &e.SrcIP, &e.User, &e.Path, &e.Message); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type IPAgg struct {
	SrcIP     string `json:"src_ip"`
	Events    int    `json:"events"`
	Fails     int    `json:"fails"`
	LastSeen  int64  `json:"last_seen"`
	Sources   string `json:"sources"`
	Banned    bool   `json:"banned"`
	BanReason string `json:"ban_reason,omitempty"`
}

var AttackFailTypes = []string{"ssh_failed", "ssh_invalid", "web_probe", "web_failed", "fail2ban_ban", "port_scan"}

func attackFailPlaceholders() string {
	return strings.Repeat("?,", len(AttackFailTypes)-1) + "?"
}

func attackFailArgs() []any {
	args := make([]any, len(AttackFailTypes))
	for i, t := range AttackFailTypes {
		args[i] = t
	}
	return args
}

func IsAttackFail(typ string) bool {
	for _, t := range AttackFailTypes {
		if t == typ {
			return true
		}
	}
	return false
}

func (s *Store) QueryIPAgg(from int64) ([]IPAgg, error) {
	args := attackFailArgs()
	args = append(args, from)
	rows, err := s.db.Query(`SELECT e.src_ip,
		COUNT(*) as events,
		SUM(CASE WHEN e.type IN (`+attackFailPlaceholders()+`) THEN 1 ELSE 0 END) as fails,
		MAX(e.ts) as last_seen,
		GROUP_CONCAT(DISTINCT e.source),
		CASE WHEN b.ip IS NULL THEN 0 ELSE 1 END as banned,
		COALESCE(b.reason,'')
		FROM security_events e
		LEFT JOIN ip_bans b ON b.ip=e.src_ip
		WHERE e.ts>=? AND e.src_ip IS NOT NULL AND e.src_ip!=''
		GROUP BY e.src_ip ORDER BY last_seen DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IPAgg{}
	for rows.Next() {
		var r IPAgg
		var banned int
		if err := rows.Scan(&r.SrcIP, &r.Events, &r.Fails, &r.LastSeen, &r.Sources, &banned, &r.BanReason); err != nil {
			return nil, err
		}
		r.Banned = banned != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CountAttackFails(from int64) ([]CountRow, error) {
	args := []any{from}
	args = append(args, attackFailArgs()...)
	q := `SELECT src_ip, COUNT(*) FROM security_events
		WHERE ts>=? AND type IN (` + attackFailPlaceholders() + `) AND src_ip!='' GROUP BY src_ip`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CountRow{}
	for rows.Next() {
		var r CountRow
		if err := rows.Scan(&r.IP, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CountAttackFailsIP(ip string, from int64) (int, error) {
	args := []any{from, ip}
	args = append(args, attackFailArgs()...)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM security_events
		WHERE ts>=? AND src_ip=? AND type IN (`+attackFailPlaceholders()+`)`, args...).Scan(&n)
	return n, err
}

type IPBan struct {
	IP        string `json:"ip"`
	CreatedAt int64  `json:"created_at"`
	Reason    string `json:"reason"`
	Hits      int    `json:"hits"`
	Auto      bool   `json:"auto"`
}

func (s *Store) InsertBan(b IPBan) (bool, error) {
	if b.CreatedAt == 0 {
		b.CreatedAt = time.Now().Unix()
	}
	existed, err := s.IsBanned(b.IP)
	if err != nil {
		return false, err
	}
	auto := 0
	if b.Auto {
		auto = 1
	}
	_, err = s.db.Exec(`INSERT INTO ip_bans(ip,created_at,reason,hits,auto) VALUES(?,?,?,?,?)
		ON CONFLICT(ip) DO UPDATE SET reason=excluded.reason, hits=excluded.hits`,
		b.IP, b.CreatedAt, b.Reason, b.Hits, auto)
	return !existed, err
}

func (s *Store) IsBanned(ip string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ip_bans WHERE ip=?`, ip).Scan(&n)
	return n > 0, err
}

func (s *Store) ListBans() ([]IPBan, error) {
	rows, err := s.db.Query(`SELECT ip,created_at,COALESCE(reason,''),hits,auto FROM ip_bans ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IPBan{}
	for rows.Next() {
		var b IPBan
		var auto int
		if err := rows.Scan(&b.IP, &b.CreatedAt, &b.Reason, &b.Hits, &auto); err != nil {
			return nil, err
		}
		b.Auto = auto != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBan(ip string) error {
	_, err := s.db.Exec(`DELETE FROM ip_bans WHERE ip=?`, ip)
	return err
}

func (s *Store) CountBans() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ip_bans`).Scan(&n)
	return n, err
}

type CountRow struct {
	IP    string
	Count int
	Extra string
}

func (s *Store) CountEvents(source string, types []string, from int64) ([]CountRow, error) {
	if len(types) == 0 {
		return nil, nil
	}
	ph := strings.Repeat("?,", len(types))
	ph = ph[:len(ph)-1]
	args := []any{from, source}
	for _, t := range types {
		args = append(args, t)
	}
	q := `SELECT src_ip, COUNT(*), COALESCE(MAX(user_name),'') FROM security_events
		WHERE ts>=? AND source=? AND type IN (` + ph + `) AND src_ip!='' GROUP BY src_ip`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CountRow{}
	for rows.Next() {
		var r CountRow
		if err := rows.Scan(&r.IP, &r.Count, &r.Extra); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CountFailsBefore(ip string, from, until int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM security_events WHERE src_ip=? AND ts>=? AND ts<=? AND type IN ('ssh_failed','ssh_invalid')`, ip, from, until).Scan(&n)
	return n, err
}

type Alert struct {
	ID        int64  `json:"id"`
	RuleID    string `json:"rule_id"`
	Key       string `json:"key"`
	Severity  string `json:"severity"`
	Status    string `json:"status"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
	Count     int    `json:"count"`
	Summary   string `json:"summary"`
}

func (s *Store) RaiseAlert(rule, key, severity, summary string) error {
	now := time.Now().Unix()
	var id int64
	err := s.db.QueryRow(`SELECT id FROM alerts WHERE rule_id=? AND key=? AND status IN ('open','acked') ORDER BY id DESC LIMIT 1`, rule, key).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec(`INSERT INTO alerts(rule_id,key,severity,status,first_seen,last_seen,count,summary) VALUES(?,?,?,?,?,?,1,?)`,
			rule, key, severity, "open", now, now, summary)
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE alerts SET last_seen=?, count=count+1, summary=?, severity=? WHERE id=?`, now, summary, severity, id)
	return err
}

func (s *Store) OpenAlerts(rule string) ([]Alert, error) {
	q := `SELECT id,rule_id,key,severity,status,first_seen,last_seen,count,summary FROM alerts WHERE status IN ('open','acked')`
	args := []any{}
	if rule != "" {
		q += ` AND rule_id=?`
		args = append(args, rule)
	}
	q += ` ORDER BY last_seen DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows)
}

func (s *Store) ListAlerts(status string, limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id,rule_id,key,severity,status,first_seen,last_seen,count,summary FROM alerts`
	args := []any{}
	if status == "active" {
		q += ` WHERE status IN ('open','acked')`
	} else if status != "" {
		q += ` WHERE status=?`
		args = append(args, status)
	}
	q += ` ORDER BY last_seen DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows)
}

func scanAlerts(rows *sql.Rows) ([]Alert, error) {
	out := []Alert{}
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.RuleID, &a.Key, &a.Severity, &a.Status, &a.FirstSeen, &a.LastSeen, &a.Count, &a.Summary); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SetAlertStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE alerts SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) SetAlertStatusMany(ids []int64, status string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph := strings.Repeat("?,", len(ids)-1) + "?"
	args := []any{status}
	for _, id := range ids {
		args = append(args, id)
	}
	q := `UPDATE alerts SET status=? WHERE id IN (` + ph + `)`
	switch status {
	case "acked":
		q += ` AND status='open'`
	case "resolved":
		q += ` AND status IN ('open','acked')`
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *Store) ResolveAlert(id int64) error { return s.SetAlertStatus(id, "resolved") }

func (s *Store) ResolveRuleKey(rule, key string) error {
	_, err := s.db.Exec(`UPDATE alerts SET status='resolved' WHERE rule_id=? AND key=? AND status IN ('open','acked')`, rule, key)
	return err
}

func (s *Store) KnownSSH(ip string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM known_ssh_ips WHERE ip=?`, ip).Scan(&n)
	return n > 0, err
}

func (s *Store) RememberSSH(ip, user string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO known_ssh_ips(ip,first_seen,last_seen,user_name) VALUES(?,?,?,?)
		ON CONFLICT(ip) DO UPDATE SET last_seen=excluded.last_seen, user_name=excluded.user_name`, ip, now, now, user)
	return err
}

func (s *Store) LoadSettings(base config.Settings) config.Settings {
	raw, ok := s.GetMeta("settings")
	if !ok || raw == "" {
		return base
	}
	cur := base
	if err := json.Unmarshal([]byte(raw), &cur); err != nil {
		return base
	}
	return cur
}

func (s *Store) SaveSettings(st config.Settings) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.SetMeta("settings", string(b))
}
