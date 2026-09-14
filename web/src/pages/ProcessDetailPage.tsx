import { useCallback } from "react";
import { Link, useSearchParams } from "react-router-dom";
import Chart from "../components/Chart";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatBytes, formatPct } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { ProcRow } from "../lib/types";

export default function ProcessDetailPage() {
  const [params] = useSearchParams();
  const name = params.get("name") || "";
  const container = params.get("container") || "";
  const fetchHist = useCallback(() => {
    const q = container ? `container=${encodeURIComponent(container)}` : `name=${encodeURIComponent(name)}`;
    return api<ProcRow[]>(`/api/v1/processes/history?range=1h&${q}`);
  }, [name, container]);
  const { data, error } = usePoll(fetchHist, 8000);
  const rows = data || [];
  const title = container || name || "进程";
  const latest = rows[rows.length - 1];

  return (
    <div className="stack">
      <PageHeader
        kicker="应用"
        title={title}
        desc={latest ? `近 1 小时 · PID ${latest.pid} · CPU ${formatPct(latest.cpu, 1)} · ${formatBytes(latest.rss)}` : "近 1 小时采样"}
      >
        <Link to="/processes" className="btn btn-ghost">
          返回列表
        </Link>
      </PageHeader>
      {error ? <div className="error">{error}</div> : null}
      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">CPU</div>
        </div>
        <Chart series={[{ name: "CPU %", color: "#3ee0b2", values: rows.map((r) => ({ t: r.ts, v: r.cpu })) }]} format={(n) => formatPct(n, 1)} />
      </article>
      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">内存 RSS</div>
        </div>
        <Chart series={[{ name: "RSS", color: "#6aa8ff", values: rows.map((r) => ({ t: r.ts, v: r.rss })) }]} format={(n) => formatBytes(n)} />
      </article>
    </div>
  );
}
