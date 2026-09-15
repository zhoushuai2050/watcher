import { useCallback } from "react";
import { Link } from "react-router-dom";
import { Sparkline } from "../components/Chart";
import Meter from "../components/Meter";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatBps, formatBytes, formatPct, formatTime, formatUptime, relTime, severityLabel } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { Overview } from "../lib/types";

export default function OverviewPage() {
  const fetchOverview = useCallback(() => api<Overview>("/api/v1/overview"), []);
  const { data, error, loading } = usePoll(fetchOverview, 5000);

  if (loading && !data) return <div className="empty">正在读取本机状态…</div>;
  if (error && !data) return <div className="error">{error}</div>;
  if (!data) return null;
  const h = data.host;
  const memPct = h.mem_total ? (h.mem_used / h.mem_total) * 100 : 0;
  const cpuSeries = data.sparkline.map((m) => m.cpu);
  const memSeries = data.sparkline.map((m) => (m.mem_total ? (m.mem_used / m.mem_total) * 100 : 0));

  return (
    <div>
      <PageHeader
        kicker={
          <>
            <span className="live-dot" />
            本机
          </>
        }
        title={h.hostname || "Watcher"}
        desc={`${h.os || "Linux"} · 已运行 ${formatUptime(h.uptime_sec)} · ${h.cpu_cores} 核${h.temp_c != null ? ` · ${h.temp_c.toFixed(0)}°C` : ""}`}
      >
        <Link to="/alerts" className="btn btn-primary" style={{ width: "auto" }}>
          {data.alerts_open} 条未关闭告警
        </Link>
      </PageHeader>
      {error ? <div className="error">{error}</div> : null}

      <div className="grid grid-4">
        <article className="card stat-card">
          <div className="kicker">CPU</div>
          <div className="metric">{formatPct(h.cpu_pct, 1)}</div>
          <Meter value={h.cpu_pct} />
          <Sparkline values={cpuSeries} />
        </article>
        <article className="card stat-card">
          <div className="kicker">内存</div>
          <div className="metric">{formatPct(memPct, 0)}</div>
          <p className="stat-sub">
            {formatBytes(h.mem_used)} / {formatBytes(h.mem_total)}
          </p>
          <Meter value={memPct} />
          <Sparkline values={memSeries} color="#6aa8ff" />
        </article>
        <article className="card stat-card">
          <div className="kicker">磁盘</div>
          <div className="metric">{formatPct(h.disk_used_pct)}</div>
          <p className="stat-sub">
            load {h.load1?.toFixed(2)} · {h.load5?.toFixed(2)} · {h.load15?.toFixed(2)}
          </p>
          <Meter value={h.disk_used_pct} />
        </article>
        <article className="card stat-card">
          <div className="kicker">网络</div>
          <div className="metric">{formatBps(h.net_in_bps)}</div>
          <p className="stat-sub">下行 · 上行 {formatBps(h.net_out_bps)}</p>
        </article>
      </div>

      <div className="grid grid-2" style={{ marginTop: 14 }}>
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">应用占用</div>
            <Link to="/resources?tab=process" className="muted">
              全部
            </Link>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>类型</th>
                  <th className="num">CPU</th>
                  <th className="num">内存</th>
                </tr>
              </thead>
              <tbody>
                {(data.groups || []).slice(0, 8).map((g) => (
                  <tr key={g.kind + g.key}>
                    <td>{g.key}</td>
                    <td>
                      <span className="chip muted">{g.kind}</span>
                    </td>
                    <td className="num">{formatPct(g.cpu, 1)}</td>
                    <td className="num">{formatBytes(g.rss)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">最近安全事件</div>
            <Link to="/security" className="muted">
              全部
            </Link>
          </div>
          <ul className="timeline">
            {(data.recent_events || []).length === 0 ? <li className="muted">还没有解析到攻击事件。</li> : null}
            {(data.recent_events || []).map((ev) => (
              <li key={ev.id}>
                <span className={`sev sev-${ev.severity}`}>{severityLabel(ev.severity)}</span>
                <div>
                  <strong>{ev.message}</strong>
                  <p className="muted">
                    <span className="ip">{ev.src_ip || "—"}</span> · {formatTime(ev.ts)} · {relTime(ev.ts)}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </article>
      </div>
    </div>
  );
}
