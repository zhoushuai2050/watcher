export function formatBytes(n: number | null | undefined) {
  if (n == null || Number.isNaN(n)) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = Math.abs(n);
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : v.toFixed(0)} ${units[i]}`;
}

export function formatBps(n: number | null | undefined) {
  if (n == null || Number.isNaN(n)) return "—";
  const units = ["B/s", "KB/s", "MB/s", "GB/s"];
  let v = Math.abs(n);
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : v.toFixed(0)} ${units[i]}`;
}

export function formatPct(n: number | null | undefined, digits = 0) {
  if (n == null || Number.isNaN(n)) return "—";
  return `${n.toFixed(digits)}%`;
}

export function formatUptime(sec: number | null | undefined) {
  if (!sec) return "—";
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分`;
  return `${m} 分钟`;
}

export function formatTime(ts: number | null | undefined) {
  if (!ts) return "—";
  const d = new Date(ts * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function relTime(ts: number | null | undefined) {
  if (!ts) return "—";
  const diff = Math.max(0, Date.now() / 1000 - ts);
  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  return `${Math.floor(diff / 86400)} 天前`;
}

export function severityLabel(s: string) {
  return ({ critical: "紧急", high: "高", medium: "中", low: "低", info: "信息" } as Record<string, string>)[s] || s;
}

export function sourceLabel(s: string) {
  return ({ ssh: "SSH", nginx: "Web", fail2ban: "Fail2ban", nft: "防火墙", net: "网络", host: "主机", watcher: "封禁" } as Record<string, string>)[s] || s;
}
