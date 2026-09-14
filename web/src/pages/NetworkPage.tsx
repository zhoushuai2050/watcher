import { useCallback } from "react";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatTime } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { NetSnapshot } from "../lib/types";

const palette = ["#3ee0b2", "#6aa8ff", "#f0c14d", "#ff6b7a", "#c084fc", "#8393a4"];

export default function NetworkPage() {
  const fetchNet = useCallback(() => api<NetSnapshot>("/api/v1/network"), []);
  const { data, error } = usePoll(fetchNet, 5000);
  const states = Object.entries(data?.states || {}).sort((a, b) => b[1] - a[1]);
  const total = states.reduce((s, [, n]) => s + n, 0) || 1;

  return (
    <div className="stack">
      <PageHeader kicker="连接" title="网络" desc="监听端口相对白名单标记；连接按对端 IP 聚合。" />
      {error ? <div className="error">{error}</div> : null}
      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">连接状态</div>
          <span className="muted">{total} 条</span>
        </div>
        <div className="state-bar">
          {states.map(([name, n], i) => (
            <span key={name} style={{ width: `${(n / total) * 100}%`, background: palette[i % palette.length] }} title={`${name} ${n}`} />
          ))}
        </div>
        <div className="state-legend">
          {states.map(([name, n], i) => (
            <span key={name}>
              <i className="swatch" style={{ background: palette[i % palette.length] }} />
              {name}
              <b>{n}</b>
            </span>
          ))}
        </div>
      </article>
      <div className="grid grid-2">
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">监听端口</div>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>端口</th>
                  <th>进程</th>
                  <th>地址</th>
                  <th>白名单</th>
                </tr>
              </thead>
              <tbody>
                {(data?.listen || []).map((p) => (
                  <tr key={`${p.proto}-${p.addr}-${p.port}`}>
                    <td className="mono">
                      {p.proto}:{p.port}
                    </td>
                    <td>
                      {p.name || "—"} <span className="muted">{p.pid || ""}</span>
                    </td>
                    <td className="muted mono">{p.addr}</td>
                    <td>{p.allowed ? <span className="chip">允许</span> : <span className="chip warn">新/未知</span>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">对端连接 Top</div>
            <span className="muted">{data?.ts ? formatTime(data.ts) : ""}</span>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>源 IP</th>
                  <th className="num">本机端口</th>
                  <th>状态</th>
                  <th className="num">数量</th>
                </tr>
              </thead>
              <tbody>
                {(data?.conns || []).slice(0, 40).map((c, i) => (
                  <tr key={i}>
                    <td className="ip">{c.src_ip}</td>
                    <td className="num">{c.dst_port}</td>
                    <td className="muted">{c.state}</td>
                    <td className="num">{c.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
      </div>
    </div>
  );
}
