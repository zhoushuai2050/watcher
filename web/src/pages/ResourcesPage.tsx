import { useCallback, useState } from "react";
import Chart from "../components/Chart";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatBps, formatPct } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { Metric } from "../lib/types";

const ranges = [
  { id: "1h", label: "1 小时" },
  { id: "6h", label: "6 小时" },
  { id: "24h", label: "24 小时" },
  { id: "7d", label: "7 天" },
];

export default function ResourcesPage() {
  const [range, setRange] = useState("1h");
  const fetchMetrics = useCallback(() => api<Metric[]>(`/api/v1/metrics?range=${range}`), [range]);
  const { data, error } = usePoll(fetchMetrics, 8000);
  const rows = data || [];

  return (
    <div className="stack">
      <PageHeader kicker="主机" title="资源曲线" desc="按时间窗口查看 CPU、内存、磁盘和网络吞吐。">
        <div className="range-tabs">
          {ranges.map((r) => (
            <button key={r.id} className={range === r.id ? "active" : ""} onClick={() => setRange(r.id)}>
              {r.label}
            </button>
          ))}
        </div>
      </PageHeader>
      {error ? <div className="error">{error}</div> : null}
      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">CPU / 负载</div>
        </div>
        <Chart
          series={[
            { name: "CPU %", color: "#3ee0b2", values: rows.map((m) => ({ t: m.ts, v: m.cpu })) },
            { name: "load1", color: "#f0c14d", values: rows.map((m) => ({ t: m.ts, v: m.load1 })) },
          ]}
          format={(n) => n.toFixed(1)}
        />
      </article>
      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">内存 / 磁盘</div>
        </div>
        <Chart
          series={[
            {
              name: "内存 %",
              color: "#6aa8ff",
              values: rows.map((m) => ({ t: m.ts, v: m.mem_total ? (m.mem_used / m.mem_total) * 100 : 0 })),
            },
            { name: "磁盘 %", color: "#c084fc", values: rows.map((m) => ({ t: m.ts, v: m.disk_used_pct })) },
          ]}
          format={(n) => formatPct(n)}
        />
      </article>
      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">网络吞吐</div>
        </div>
        <Chart
          series={[
            { name: "下行", color: "#3ee0b2", values: rows.map((m) => ({ t: m.ts, v: m.net_in_bps })) },
            { name: "上行", color: "#ff6b7a", values: rows.map((m) => ({ t: m.ts, v: m.net_out_bps })) },
          ]}
          format={(n) => formatBps(n)}
        />
      </article>
    </div>
  );
}
