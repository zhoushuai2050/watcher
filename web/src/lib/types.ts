export type User = { id: string | number; name: string };

export type Page<T> = {
  items: T[];
  total: number;
  page: number;
  page_size: number;
};

export type DiskInfo = {
  mount: string;
  fstype: string;
  used: number;
  total: number;
  pct: number;
};

export type HostSample = {
  hostname: string;
  os: string;
  uptime_sec: number;
  cpu_pct: number;
  cpu_cores: number;
  mem_used: number;
  mem_total: number;
  swap_used: number;
  swap_total: number;
  load1: number;
  load5: number;
  load15: number;
  disk_used_pct: number;
  net_in_bps: number;
  net_out_bps: number;
  disk_read_bps: number;
  disk_write_bps: number;
  temp_c: number | null;
  disks: DiskInfo[] | null;
};

export type Metric = {
  ts: number;
  cpu: number;
  mem_used: number;
  mem_total: number;
  load1: number;
  disk_used_pct: number;
  net_in_bps: number;
  net_out_bps: number;
  disk_read_bps: number;
  disk_write_bps: number;
  temp_c: number | null;
};

export type ProcRow = {
  ts: number;
  pid: number;
  name: string;
  unit: string;
  container: string;
  user: string;
  cpu: number;
  rss: number;
  read_bytes: number;
  write_bytes: number;
  cmdline: string;
};

export type ProcGroup = {
  key: string;
  kind: string;
  cpu: number;
  rss: number;
  count: number;
};

export type ListenPort = {
  proto: string;
  family: string;
  addr: string;
  port: number;
  pid: number;
  name: string;
  first_seen: number;
  last_seen: number;
  allowed: boolean;
};

export type ConnRow = {
  src_ip: string;
  dst_port: number;
  state: string;
  count: number;
};

export type IPLookup = {
  ip: string;
  family: string;
  private: boolean;
  country: string;
  region: string;
  city: string;
  isp: string;
  org: string;
  asn: string;
  timezone: string;
  lat: number;
  lon: number;
  location: string;
  source: string;
};

export type NetSnapshot = {
  ts: number;
  listen: ListenPort[];
  conns: ConnRow[];
  states: Record<string, number>;
};

export type Event = {
  id: number;
  ts: number;
  source: string;
  type: string;
  severity: string;
  src_ip: string;
  user: string;
  path: string;
  message: string;
};

export type IPAgg = {
  src_ip: string;
  events: number;
  fails: number;
  last_seen: number;
  sources: string;
  banned: boolean;
  ban_reason?: string;
  whitelisted: boolean;
};

export type IPAllow = {
  spec: string;
  note: string;
  created_at: number;
};

export type IPBan = {
  ip: string;
  created_at: number;
  reason: string;
  hits: number;
  auto: boolean;
};

export type Alert = {
  id: number;
  rule_id: string;
  key: string;
  severity: string;
  status: string;
  first_seen: number;
  last_seen: number;
  count: number;
  summary: string;
};

export type CollectorStatus = {
  name: string;
  enabled: boolean;
  ok: boolean;
  last_run: number;
  last_error: string;
  detail: string;
};

export type Overview = {
  host: HostSample;
  alerts_open: number;
  alerts: Alert[];
  recent_events: Event[];
  top_processes: ProcRow[];
  groups: ProcGroup[];
  sparkline: Metric[];
  collectors: CollectorStatus[];
};

export type Settings = {
  cpu_pct: number;
  mem_pct: number;
  disk_pct: number;
  cpu_sustain_sec: number;
  ssh_fail_count: number;
  ssh_fail_window_sec: number;
  invalid_user_count: number;
  web_probe_count: number;
  web_probe_window_sec: number;
  web_fail_count: number;
  port_scan_ports: number;
  port_scan_window_sec: number;
  conn_flood_count: number;
  syn_recv_count: number;
  proc_cpu_pct: number;
  listen_allow: string[];
  auth_log: string;
  nginx_log: string;
  fail2ban_log: string;
  attack_ban_threshold: number;
};
