import { FormEvent, useCallback, useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import SearchPager, { useQueryPage } from "../components/SearchPager";
import { api } from "../lib/api";
import { formatTime, relTime, severityLabel, sourceLabel } from "../lib/format";
import { usePoll } from "../lib/poll";
import type { Event, IPAgg, IPAllow, IPBan, Page, Settings } from "../lib/types";

const PAGE_SIZE = 20;
const TIMELINE_SIZE = 200;

export default function SecurityPage() {
  const [tab, setTab] = useState<"events" | "allow" | "bans">("events");
  const bansPager = useQueryPage();
  const ipsPager = useQueryPage();
  const eventsPager = useQueryPage();
  const allowPager = useQueryPage();
  const [source, setSource] = useState("");
  const [ip, setIp] = useState("");
  const [threshold, setThreshold] = useState(3);
  const [saved, setSaved] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState("");
  const [allowSpec, setAllowSpec] = useState("");
  const [allowNote, setAllowNote] = useState("");

  const fetchEvents = useCallback(
    () =>
      api<Page<Event>>(
        `/api/v1/security/events?page=1&page_size=${TIMELINE_SIZE}&q=${encodeURIComponent(eventsPager.q)}&source=${encodeURIComponent(source)}&ip=${encodeURIComponent(ip)}`,
      ),
    [source, ip, eventsPager.q],
  );
  const fetchIps = useCallback(
    () => api<Page<IPAgg>>(`/api/v1/security/ips?page=${ipsPager.page}&page_size=${PAGE_SIZE}&q=${encodeURIComponent(ipsPager.q)}`),
    [ipsPager.page, ipsPager.q],
  );
  const fetchBans = useCallback(
    () => api<Page<IPBan>>(`/api/v1/security/bans?page=${bansPager.page}&page_size=${PAGE_SIZE}&q=${encodeURIComponent(bansPager.q)}`),
    [bansPager.page, bansPager.q],
  );
  const fetchAllow = useCallback(
    () => api<Page<IPAllow>>(`/api/v1/security/allow?page=${allowPager.page}&page_size=${PAGE_SIZE}&q=${encodeURIComponent(allowPager.q)}`),
    [allowPager.page, allowPager.q],
  );
  const events = usePoll(fetchEvents, 5000);
  const ips = usePoll(fetchIps, 8000);
  const bans = usePoll(fetchBans, 8000);
  const allow = usePoll(fetchAllow, 8000);

  useEffect(() => {
    void api<{ settings: Settings }>("/api/v1/settings")
      .then((d) => setThreshold(d.settings.attack_ban_threshold ?? 3))
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

  async function addAllow(spec: string, note: string) {
    const target = spec.trim();
    if (!target) return;
    setBusy(target);
    setActionError("");
    setSaved("");
    try {
      await api("/api/v1/security/allow", { method: "POST", body: JSON.stringify({ spec: target, note }) });
      setAllowSpec("");
      setAllowNote("");
      await Promise.all([allow.refresh(), ips.refresh(), bans.refresh()]);
      setSaved("已加入白名单");
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "添加失败");
    } finally {
      setBusy("");
    }
  }

  async function removeAllow(spec: string) {
    setBusy(spec);
    setActionError("");
    try {
      await api("/api/v1/security/allow/delete", { method: "POST", body: JSON.stringify({ spec }) });
      await Promise.all([allow.refresh(), ips.refresh()]);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "删除失败");
    } finally {
      setBusy("");
    }
  }

  const thresh = Number.isFinite(Number(threshold)) ? Number(threshold) : 3;
  const banItems = bans.data?.items || [];
  const ipItems = ips.data?.items || [];
  const eventItems = events.data?.items || [];
  const allowItems = allow.data?.items || [];

  return (
    <div className="stack">
      <PageHeader
        kicker="防护"
        title={tab === "allow" ? "白名单" : tab === "bans" ? "永久封禁" : "安全"}
        desc={
          tab === "allow"
            ? "白名单内的 IP 不会被封，可填单个地址或 CIDR。"
            : tab === "bans"
              ? "已永久封禁的 IP，可搜索、解封或改加入白名单。"
              : "失败次数达到阈值会永久封禁。列表可搜索、分页。"
        }
      >
        <div className="range-tabs">
          <button className={tab === "events" ? "active" : ""} onClick={() => setTab("events")}>
            攻击观测
          </button>
          <button className={tab === "allow" ? "active" : ""} onClick={() => setTab("allow")}>
            白名单
          </button>
          <button className={tab === "bans" ? "active" : ""} onClick={() => setTab("bans")}>
            永久封禁{bans.data?.total ? ` ${bans.data.total}` : ""}
          </button>
        </div>
      </PageHeader>
      {events.error ? <div className="error">{events.error}</div> : null}
      {actionError ? <div className="error">{actionError}</div> : null}
      {saved ? <div className="ok">{saved}</div> : null}

      {tab === "bans" ? (
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">永久封禁名单</div>
            <span className="muted">{bans.data?.total || 0} 个 IP</span>
          </div>
          <SearchPager
            input={bansPager.input}
            onInput={bansPager.setInput}
            placeholder="模糊搜索 IP 或原因"
            page={bansPager.page}
            pageSize={PAGE_SIZE}
            total={bans.data?.total || 0}
            onPage={bansPager.setPage}
          />
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
                {banItems.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="muted">
                      {(bans.data?.total || 0) === 0 ? "当前没有永久封禁的 IP。" : "没有匹配的记录。"}
                    </td>
                  </tr>
                ) : null}
                {banItems.map((b) => (
                    <tr key={b.ip}>
                      <td className="ip">{b.ip}</td>
                      <td className="muted">{b.reason || "—"}</td>
                      <td>{b.auto ? "自动" : "手动"}</td>
                      <td className="muted">{formatTime(b.created_at)}</td>
                      <td>
                        <div className="inline-actions">
                          <button className="btn btn-ghost" disabled={busy === b.ip} onClick={() => void addAllow(b.ip, "")}>
                            白名单
                          </button>
                          <button className="btn btn-ghost" disabled={busy === b.ip} onClick={() => void unbanIP(b.ip)}>
                            解封
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        </article>
      ) : tab === "allow" ? (
        <article className="card panel">
          <div className="panel-head">
            <div className="kicker">允许访问的 IP</div>
            <span className="muted">{allow.data?.total || 0} 条</span>
          </div>
          <p className="hint-inline" style={{ marginBottom: 14 }}>
            白名单中的地址不会被自动或手动永久封禁；加入时若已封会自动解封。可填单个 IP 或 CIDR，例如 203.0.113.10、198.51.100.0/24。
          </p>
          <SearchPager
            input={allowPager.input}
            onInput={allowPager.setInput}
            placeholder="模糊搜索 IP 或备注"
            page={allowPager.page}
            pageSize={PAGE_SIZE}
            total={allow.data?.total || 0}
            onPage={allowPager.setPage}
          />
          <form
            className="threshold-form"
            onSubmit={(e) => {
              e.preventDefault();
              void addAllow(allowSpec, allowNote);
            }}
          >
            <div className="field grow">
              <label>IP / CIDR</label>
              <input value={allowSpec} onChange={(e) => setAllowSpec(e.target.value)} placeholder="203.0.113.10 或 198.51.100.0/24" />
            </div>
            <div className="field grow">
              <label>备注</label>
              <input value={allowNote} onChange={(e) => setAllowNote(e.target.value)} placeholder="例如 家里出口" />
            </div>
            <button className="btn btn-primary" style={{ width: "auto" }} disabled={!allowSpec.trim() || busy === allowSpec.trim()}>
              添加
            </button>
          </form>
          <div className="table-wrap" style={{ marginTop: 16 }}>
            <table className="table">
              <thead>
                <tr>
                  <th>IP / CIDR</th>
                  <th>备注</th>
                  <th>加入时间</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {allowItems.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="muted">
                      {(allow.data?.total || 0) === 0 ? "还没有白名单。本机、内网和 Cloudflare 回源默认就不会被自动封。" : "没有匹配的记录。"}
                    </td>
                  </tr>
                ) : null}
                {allowItems.map((a) => (
                  <tr key={a.spec}>
                    <td className="ip">{a.spec}</td>
                    <td className="muted">{a.note || "—"}</td>
                    <td className="muted">{formatTime(a.created_at)}</td>
                    <td>
                      <button className="btn btn-ghost" disabled={busy === a.spec} onClick={() => void removeAllow(a.spec)}>
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
      ) : (
        <>
          <article className="card panel">
            <div className="panel-head">
              <div className="kicker">攻击阈值 · 永久封禁</div>
              <span className="muted">{bans.data?.total || 0} 个 IP 已永久封禁</span>
            </div>
            <form className="threshold-form" onSubmit={saveThreshold}>
              <div className="field">
                <label>攻击阈值</label>
                <input type="number" min={0} max={10000} value={threshold} onChange={(e) => setThreshold(Number(e.target.value))} />
              </div>
              <button className="btn btn-primary" style={{ width: "auto" }}>
                保存阈值
              </button>
              <p className="hint-inline">
                同一 IP 在 7 天内失败类事件（SSH 失败、Web 探测、Fail2ban 封禁等）≥ {thresh || 0}{" "}
                次则永久封禁。0 表示关闭自动封禁。本机、内网和 Cloudflare 回源不会自动封。常用出口请加到白名单。
              </p>
            </form>
          </article>

          <div className="grid grid-2">
            <article className="card panel">
              <div className="panel-head">
                <div className="kicker">按 IP 聚合 · 7 天</div>
              </div>
              <SearchPager
                input={ipsPager.input}
                onInput={ipsPager.setInput}
                placeholder="模糊搜索 IP / 来源 / 原因"
                page={ipsPager.page}
                pageSize={PAGE_SIZE}
                total={ips.data?.total || 0}
                onPage={ipsPager.setPage}
              />
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
                    {ipItems.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="muted">
                          窗口内没有 IP 记录。
                        </td>
                      </tr>
                    ) : null}
                    {ipItems.map((row) => {
                      const over = thresh > 0 && row.fails >= thresh;
                      return (
                        <tr key={row.src_ip} className={ip === row.src_ip ? "row-active" : ""}>
                          <td className="ip">{row.src_ip}</td>
                          <td className="num">{row.events}</td>
                          <td className="num">{row.fails}</td>
                          <td>
                            {row.whitelisted ? (
                              <span className="chip">白名单</span>
                            ) : row.banned ? (
                              <span className="chip danger">永久封禁</span>
                            ) : over ? (
                              <span className="chip warn">超过阈值</span>
                            ) : (
                              <span className="muted">{row.sources}</span>
                            )}
                          </td>
                          <td>
                            <div className="inline-actions">
                              <button
                                className="btn btn-ghost"
                                onClick={() => setIp(row.src_ip === ip ? "" : row.src_ip)}
                              >
                                {row.src_ip === ip ? "取消" : "筛选"}
                              </button>
                              {row.whitelisted ? (
                                <button className="btn btn-ghost" disabled={busy === row.src_ip} onClick={() => void removeAllow(row.src_ip)}>
                                  移出白名单
                                </button>
                              ) : (
                                <button className="btn btn-ghost" disabled={busy === row.src_ip} onClick={() => void addAllow(row.src_ip, "")}>
                                  白名单
                                </button>
                              )}
                              {row.banned ? (
                                <button className="btn btn-ghost" disabled={busy === row.src_ip} onClick={() => void unbanIP(row.src_ip)}>
                                  解封
                                </button>
                              ) : !row.whitelisted ? (
                                <button className="btn btn-danger" disabled={busy === row.src_ip} onClick={() => void banIP(row.src_ip)}>
                                  永久封禁
                                </button>
                              ) : null}
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
                <span className="muted">{events.data?.total || 0} 条</span>
              </div>
              <div className="range-tabs" style={{ marginBottom: 12 }}>
                {["", "ssh", "nginx", "fail2ban", "nft", "watcher", "net"].map((s) => (
                  <button key={s || "all"} className={source === s ? "active" : ""} onClick={() => setSource(s)}>
                    {s ? sourceLabel(s) : "全部"}
                  </button>
                ))}
              </div>
              <SearchPager
                input={eventsPager.input}
                onInput={eventsPager.setInput}
                placeholder="模糊搜索 IP / 内容 / 路径"
                page={1}
                pageSize={TIMELINE_SIZE}
                total={events.data?.total || 0}
                onPage={() => undefined}
                hideNav
              />
              <ul className="timeline">
                {eventItems.length === 0 ? <li className="muted">窗口内没有事件。采集器读不到日志时去设置页看状态。</li> : null}
                {eventItems.map((ev) => (
                  <li key={ev.id}>
                    <span className={`sev sev-${ev.severity}`}>{severityLabel(ev.severity)}</span>
                    <div>
                      <strong title={ev.message}>{ev.message}</strong>
                      <p
                        className="muted"
                        title={`${sourceLabel(ev.source)} · ${ev.src_ip || "—"}${ev.user ? ` · ${ev.user}` : ""}${ev.path ? ` · ${ev.path}` : ""} · ${formatTime(ev.ts)}`}
                      >
                        <span className="chip muted">{sourceLabel(ev.source)}</span> · <span className="ip">{ev.src_ip || "—"}</span>
                        {ev.user ? ` · ${ev.user}` : ""} {ev.path ? ` · ${ev.path}` : ""} · {formatTime(ev.ts)} · {relTime(ev.ts)}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            </article>
          </div>
        </>
      )}
    </div>
  );
}
