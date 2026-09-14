import { FormEvent, useCallback, useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatTime, relTime, severityLabel, sourceLabel } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { Event, IPAgg, IPBan, Settings } from "../lib/types";

export default function SecurityPage() {
  const [source, setSource] = useState("");
  const [ip, setIp] = useState("");
  const [threshold, setThreshold] = useState(3);
  const [ignore, setIgnore] = useState("");
  const [saved, setSaved] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState("");

  const fetchEvents = useCallback(
    () => api<Event[]>(`/api/v1/security/events?limit=120&source=${encodeURIComponent(source)}&ip=${encodeURIComponent(ip)}`),
    [source, ip],
  );
  const fetchIps = useCallback(() => api<IPAgg[]>("/api/v1/security/ips"), []);
  const fetchBans = useCallback(() => api<IPBan[]>("/api/v1/security/bans"), []);
  const events = usePoll(fetchEvents, 5000);
  const ips = usePoll(fetchIps, 8000);
  const bans = usePoll(fetchBans, 8000);

  useEffect(() => {
    void api<{ settings: Settings }>("/api/v1/settings")
      .then((d) => {
        setThreshold(d.settings.attack_ban_threshold ?? 3);
        setIgnore((d.settings.ban_ignore || []).join(", "));
      })
      .catch(() => undefined);
  }, []);

  async function saveThreshold(e: FormEvent) {
    e.preventDefault();
    setActionError("");
    setSaved("");
    try {
      const cur = await api<{ settings: Settings }>("/api/v1/settings");
      const n = Number(threshold);
      await api("/api/v1/settings", {
        method: "PUT",
        body: JSON.stringify({
          ...cur.settings,
          attack_ban_threshold: Number.isFinite(n) ? n : 3,
          ban_ignore: ignore
            .split(/[,\s]+/)
            .map((s) => s.trim())
            .filter(Boolean),
        }),
      });
      setSaved("阈值已保存，正在按新规则扫描");
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "保存失败");
    }
  }

  async function banIP(target: string) {
    setBusy(target);
    setActionError("");
    try {
      await api("/api/v1/security/bans", { method: "POST", body: JSON.stringify({ ip: target, reason: "安全页手动封禁" }) });
      await Promise.all([ips.refresh(), bans.refresh()]);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "封禁失败");
    } finally {
      setBusy("");
    }
  }

  async function unbanIP(target: string) {
    setBusy(target);
    setActionError("");
    try {
      await api("/api/v1/security/unban", { method: "POST", body: JSON.stringify({ ip: target }) });
      await Promise.all([ips.refresh(), bans.refresh()]);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "解封失败");
    } finally {
      setBusy("");
    }
  }

  const thresh = Number.isFinite(Number(threshold)) ? Number(threshold) : 3;

  return (
    <div className="stack">
      <PageHeader kicker="攻击观测" title="安全" desc="SSH 爆破、Web 探测、Fail2ban 封禁。失败类事件达到阈值后 nftables 永久全端口丢弃。">
        <div className="range-tabs">
          {["", "ssh", "nginx", "fail2ban", "nft", "watcher", "net"].map((s) => (
            <button key={s || "all"} className={source === s ? "active" : ""} onClick={() => setSource(s)}>
              {s ? sourceLabel(s) : "全部"}
            </button>
          ))}
        </div>
      </PageHeader>
      {events.error ? <div className="error">{events.error}</div> : null}
      {actionError ? <div className="error">{actionError}</div> : null}
      {saved ? <div className="ok">{saved}</div> : null}

      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">攻击阈值 · 永久封禁</div>
          <span className="muted">{(bans.data || []).length} 个 IP 已永久封禁</span>
        </div>
        <form className="threshold-form" onSubmit={saveThreshold}>
          <div className="field">
            <label>攻击阈值</label>
            <input
              type="number"
              min={0}
              max={10000}
              value={threshold}
              onChange={(e) => setThreshold(Number(e.target.value))}
            />
          </div>
          <div className="field grow">
            <label>忽略 IP / CIDR（不自动封）</label>
            <input value={ignore} onChange={(e) => setIgnore(e.target.value)} placeholder="例如 203.0.113.10, 198.51.100.0/24" />
          </div>
          <button className="btn btn-primary" style={{ width: "auto" }}>
            保存阈值
          </button>
          <p className="hint-inline">
            同一 IP 在 7 天内失败类事件（SSH 失败、Web 探测、Fail2ban 封禁等）≥ {thresh || 0}{" "}
            次则永久封禁。0 表示关闭自动封禁。本机、内网和 Cloudflare 回源不会自动封。
          </p>
        </form>
      </article>

      <div className="grid grid-2">
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">按 IP 聚合 · 7 天</div>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th className="num">事件</th>
                  <th className="num">失败类</th>
                  <th>状态</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {(ips.data || []).map((row) => {
                  const over = thresh > 0 && row.fails >= thresh;
                  return (
                    <tr key={row.src_ip} className={ip === row.src_ip ? "row-active" : ""}>
                      <td className="ip">{row.src_ip}</td>
                      <td className="num">{row.events}</td>
                      <td className="num">{row.fails}</td>
                      <td>
                        {row.banned ? (
                          <span className="chip danger">永久封禁</span>
                        ) : over ? (
                          <span className="chip warn">超过阈值</span>
                        ) : (
                          <span className="muted">{row.sources}</span>
                        )}
                      </td>
                      <td>
                        <div className="inline-actions">
                          <button className="btn btn-ghost" onClick={() => setIp(row.src_ip === ip ? "" : row.src_ip)}>
                            {row.src_ip === ip ? "取消" : "筛选"}
                          </button>
                          {row.banned ? (
                            <button className="btn btn-ghost" disabled={busy === row.src_ip} onClick={() => void unbanIP(row.src_ip)}>
                              解封
                            </button>
                          ) : (
                            <button className="btn btn-danger" disabled={busy === row.src_ip} onClick={() => void banIP(row.src_ip)}>
                              永久封禁
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </article>
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">时间线{ip ? ` · ${ip}` : ""}</div>
          </div>
          <ul className="timeline">
            {(events.data || []).length === 0 ? <li className="muted">窗口内没有事件。采集器读不到日志时去设置页看状态。</li> : null}
            {(events.data || []).map((ev) => (
              <li key={ev.id}>
                <span className={`sev sev-${ev.severity}`}>{severityLabel(ev.severity)}</span>
                <div>
                  <strong>{ev.message}</strong>
                  <p className="muted">
                    <span className="chip muted">{sourceLabel(ev.source)}</span> · <span className="ip">{ev.src_ip || "—"}</span>
                    {ev.user ? ` · ${ev.user}` : ""} {ev.path ? ` · ${ev.path}` : ""} · {formatTime(ev.ts)} · {relTime(ev.ts)}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </article>
      </div>

      {(bans.data || []).length > 0 ? (
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">永久封禁名单</div>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th>原因</th>
                  <th>方式</th>
                  <th>时间</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {(bans.data || []).map((b) => (
                  <tr key={b.ip}>
                    <td className="ip">{b.ip}</td>
                    <td className="muted">{b.reason || "—"}</td>
                    <td>{b.auto ? "自动" : "手动"}</td>
                    <td className="muted">{formatTime(b.created_at)}</td>
                    <td>
                      <button className="btn btn-ghost" disabled={busy === b.ip} onClick={() => void unbanIP(b.ip)}>
                        解封
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
      ) : null}
    </div>
  );
}
