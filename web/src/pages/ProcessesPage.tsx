import { useCallback, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { formatBytes, formatPct } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { ProcGroup, ProcRow } from "../lib/types";

function hrefFor(name: string, container: string) {
  const q = container ? `container=${encodeURIComponent(container)}` : `name=${encodeURIComponent(name)}`;
  return `/resources/view?${q}`;
}

type Payload = { items: ProcRow[]; groups: ProcGroup[] };

export default function ProcessesPage() {
  const fetchProc = useCallback(() => api<Payload>("/api/v1/processes"), []);
  const { data, error } = usePoll(fetchProc, 5000);
  const [sort, setSort] = useState<"cpu" | "rss">("cpu");
  const items = useMemo(() => {
    const list = [...(data?.items || [])];
    list.sort((a, b) => (sort === "cpu" ? b.cpu - a.cpu : b.rss - a.rss));
    return list;
  }, [data, sort]);

  return (
    <div className="stack">
      <div className="range-tabs">
        <button className={sort === "cpu" ? "active" : ""} onClick={() => setSort("cpu")}>
          CPU
        </button>
        <button className={sort === "rss" ? "active" : ""} onClick={() => setSort("rss")}>
          内存
        </button>
      </div>
      {error ? <div className="error">{error}</div> : null}
      <div className="grid grid-2">
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">聚合 · 服务 / 容器</div>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>类型</th>
                  <th className="num">CPU</th>
                  <th className="num">内存</th>
                  <th className="num">进程</th>
                </tr>
              </thead>
              <tbody>
                {(data?.groups || []).map((g) => (
                  <tr key={g.kind + g.key}>
                    <td>
                      <Link to={hrefFor(g.kind === "container" ? "" : g.key, g.kind === "container" ? g.key : "")}>{g.key}</Link>
                    </td>
                    <td>
                      <span className="chip muted">{g.kind}</span>
                    </td>
                    <td className="num">{formatPct(g.cpu, 1)}</td>
                    <td className="num">{formatBytes(g.rss)}</td>
                    <td className="num">{g.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">Top 进程</div>
          </div>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>进程</th>
                  <th className="num">PID</th>
                  <th className="num">CPU</th>
                  <th className="num">RSS</th>
                </tr>
              </thead>
              <tbody>
                {items.map((p) => (
                  <tr key={p.pid}>
                    <td>
                      <Link to={hrefFor(p.name, p.container)}>{p.name}</Link>
                      <div className="muted">{p.container || p.unit || p.user}</div>
                    </td>
                    <td className="num muted">{p.pid}</td>
                    <td className="num">{formatPct(p.cpu, 1)}</td>
                    <td className="num">{formatBytes(p.rss)}</td>
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
