import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatTime, severityLabel } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { Alert } from "../lib/types";

export default function AlertsPage() {
  const [status, setStatus] = useState("active");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const selectAllRef = useRef<HTMLInputElement>(null);
  const fetchAlerts = useCallback(() => api<Alert[]>(`/api/v1/alerts?status=${status}`), [status]);
  const alerts = usePoll(fetchAlerts, 4000);
  const rows = alerts.data || [];
  const selectable = useMemo(() => rows.filter((a) => a.status !== "resolved"), [rows]);
  const selectableIds = useMemo(() => selectable.map((a) => a.id), [selectable]);
  const selectableKey = selectableIds.join(",");
  const selectedIds = selectableIds.filter((id) => selected.has(id));
  const allChecked = selectableIds.length > 0 && selectedIds.length === selectableIds.length;
  const someChecked = selectedIds.length > 0 && !allChecked;

  useEffect(() => {
    setSelected(new Set());
    setError("");
  }, [status]);

  useEffect(() => {
    const live = new Set(selectableKey ? selectableKey.split(",").map(Number) : []);
    setSelected((prev) => {
      const next = new Set<number>();
      prev.forEach((id) => {
        if (live.has(id)) next.add(id);
      });
      if (next.size === prev.size) {
        let same = true;
        prev.forEach((id) => {
          if (!next.has(id)) same = false;
        });
        if (same) return prev;
      }
      return next;
    });
  }, [selectableKey]);

  useEffect(() => {
    if (selectAllRef.current) selectAllRef.current.indeterminate = someChecked;
  }, [someChecked]);

  function toggleOne(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (allChecked) setSelected(new Set());
    else setSelected(new Set(selectableIds));
  }

  async function act(id: number, kind: "ack" | "resolve") {
    setError("");
    try {
      await api(`/api/v1/alerts/${id}/${kind}`, { method: "POST" });
      await alerts.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败");
    }
  }

  async function bulk(kind: "ack" | "resolve") {
    if (selectedIds.length === 0 || busy) return;
    setBusy(true);
    setError("");
    try {
      await api(`/api/v1/alerts/${kind}`, { method: "POST", body: JSON.stringify({ ids: selectedIds }) });
      setSelected(new Set());
      await alerts.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  const canAck = selectedIds.some((id) => rows.find((a) => a.id === id)?.status === "open");
  const showSelect = status === "active";

  return (
    <div>
      <PageHeader kicker="规则" title="告警" desc="条件消失会自动关闭；也可全选后批量确认或关闭。">
        <div className="range-tabs">
          <button className={status === "active" ? "active" : ""} onClick={() => setStatus("active")}>
            未关闭
          </button>
          <button className={status === "resolved" ? "active" : ""} onClick={() => setStatus("resolved")}>
            已解决
          </button>
        </div>
      </PageHeader>
      {alerts.error ? <div className="error">{alerts.error}</div> : null}
      {error ? <div className="error">{error}</div> : null}
      <article className="card panel">
        {showSelect ? (
          <div className="bulk-bar">
            <button className="btn btn-ghost" disabled={selectableIds.length === 0 || busy} onClick={toggleAll}>
              {allChecked ? "取消全选" : "全选"}
            </button>
            <span className="muted">{selectedIds.length ? `已选 ${selectedIds.length} 条` : `共 ${selectableIds.length} 条`}</span>
            <button className="btn btn-ghost" disabled={!canAck || busy} onClick={() => void bulk("ack")}>
              确认
            </button>
            <button className="btn btn-ghost" disabled={selectedIds.length === 0 || busy} onClick={() => void bulk("resolve")}>
              关闭
            </button>
          </div>
        ) : null}
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                {showSelect ? (
                  <th className="check">
                    <input
                      ref={selectAllRef}
                      type="checkbox"
                      checked={allChecked}
                      disabled={selectableIds.length === 0}
                      onChange={toggleAll}
                      aria-label="全选"
                    />
                  </th>
                ) : null}
                <th>级别</th>
                <th>规则</th>
                <th>对象</th>
                <th>摘要</th>
                <th className="num">次数</th>
                <th>最近</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={showSelect ? 8 : 7} className="muted">
                    当前没有告警。
                  </td>
                </tr>
              ) : null}
              {rows.map((a) => (
                <tr key={a.id} className={selected.has(a.id) ? "row-active" : ""}>
                  {showSelect ? (
                    <td className="check">
                      <input
                        type="checkbox"
                        checked={selected.has(a.id)}
                        onChange={() => toggleOne(a.id)}
                        aria-label={`选择 ${a.rule_id} ${a.key}`}
                      />
                    </td>
                  ) : null}
                  <td>
                    <span className={`sev sev-${a.severity}`}>{severityLabel(a.severity)}</span>
                  </td>
                  <td className="mono">{a.rule_id}</td>
                  <td className="ip">{a.key}</td>
                  <td>{a.summary}</td>
                  <td className="num">{a.count}</td>
                  <td className="muted">{formatTime(a.last_seen)}</td>
                  <td>
                    {a.status === "open" ? (
                      <button className="btn btn-ghost" onClick={() => void act(a.id, "ack")}>
                        确认
                      </button>
                    ) : null}{" "}
                    {a.status !== "resolved" ? (
                      <button className="btn btn-ghost" onClick={() => void act(a.id, "resolve")}>
                        关闭
                      </button>
                    ) : (
                      <span className="muted">已解决</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </article>
    </div>
  );
}
