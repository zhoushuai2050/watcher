import { FormEvent, useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import { api } from "../lib/api";
import { formatTime } from "../lib/format";
import type { CollectorStatus, Settings } from "../lib/types";

type Payload = { settings: Settings; collectors: CollectorStatus[]; default_password: boolean };

export default function SettingsPage() {
  const [data, setData] = useState<Payload | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [allow, setAllow] = useState("");
  const [oldP, setOldP] = useState("");
  const [newP, setNewP] = useState("");

  async function load() {
    try {
      const next = await api<Payload>("/api/v1/settings");
      setData(next);
      setAllow((next.settings.listen_allow || []).join(", "));
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!data) return;
    setError("");
    setSaved("");
    try {
      const settings = {
        ...data.settings,
        listen_allow: allow
          .split(/[,\s]+/)
          .map((s) => s.trim())
          .filter(Boolean),
      };
      await api("/api/v1/settings", { method: "PUT", body: JSON.stringify(settings) });
      setSaved("已保存");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  }

  async function onPass(e: FormEvent) {
    e.preventDefault();
    setError("");
    setSaved("");
    try {
      await api("/api/v1/me/password", { method: "POST", body: JSON.stringify({ old_password: oldP, new_password: newP }) });
      setSaved("密码已更新");
      setOldP("");
      setNewP("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "修改失败");
    }
  }

  function num(key: keyof Settings, label: string) {
    if (!data) return null;
    return (
      <div className="field">
        <label>{label}</label>
        <input
          type="number"
          value={data.settings[key] as number}
          onChange={(e) =>
            setData({
              ...data,
              settings: { ...data.settings, [key]: Number(e.target.value) },
            })
          }
        />
      </div>
    );
  }

  if (!data) return error ? <div className="error">{error}</div> : <div className="empty">加载设置…</div>;

  return (
    <div className="stack">
      <PageHeader kicker="本机" title="设置" desc="采集器读不到日志时会在这里标红，而不是静默空白。" />
      {error ? <div className="error">{error}</div> : null}
      {saved ? <div className="ok">{saved}</div> : null}
      {data.default_password ? <div className="error">仍在使用默认密码 watcher，请尽快改掉。</div> : null}

      <article className="card panel">
        <div className="panel-head">
          <div className="kicker">采集器状态</div>
        </div>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>名称</th>
                <th>状态</th>
                <th>详情</th>
                <th>最近</th>
              </tr>
            </thead>
            <tbody>
              {data.collectors.map((c) => (
                <tr key={c.name}>
                  <td className="mono">{c.name}</td>
                  <td>{c.ok ? <span className="chip">正常</span> : <span className="chip warn">异常</span>}</td>
                  <td className="muted">{c.detail || c.last_error || "—"}</td>
                  <td className="muted">{formatTime(c.last_run)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="muted" style={{ marginTop: 12, fontSize: 12 }}>
          读不到 /var/log/auth.log 时，把运行用户加入 adm 组，或用 journalctl。Nginx 日志同理。
        </p>
      </article>

      <form className="card panel" onSubmit={onSave}>
        <div className="panel-head">
          <div className="kicker">阈值与路径</div>
        </div>
        <div className="grid grid-3">
          {num("cpu_pct", "CPU 告警 %")}
          {num("mem_pct", "内存告警 %")}
          {num("disk_pct", "磁盘告警 %")}
          {num("ssh_fail_count", "SSH 失败次数")}
          {num("ssh_fail_window_sec", "SSH 窗口（秒）")}
          {num("invalid_user_count", "无效用户次数")}
          {num("web_probe_count", "Web 探测次数")}
          {num("web_fail_count", "Web 登录失败次数")}
          {num("port_scan_ports", "端口扫描端口数")}
          {num("conn_flood_count", "单 IP 连接洪水")}
          {num("syn_recv_count", "SYN_RECV 阈值")}
          {num("proc_cpu_pct", "进程 CPU 告警 %")}
        </div>
        <div className="field">
          <label>监听白名单（proto:port，逗号分隔）</label>
          <input value={allow} onChange={(e) => setAllow(e.target.value)} />
        </div>
        <div className="row-2">
          <div className="field">
            <label>Auth 日志</label>
            <input
              value={data.settings.auth_log || ""}
              onChange={(e) => setData({ ...data, settings: { ...data.settings, auth_log: e.target.value } })}
              placeholder="/var/log/auth.log"
            />
          </div>
          <div className="field">
            <label>Nginx access.log</label>
            <input
              value={data.settings.nginx_log || ""}
              onChange={(e) => setData({ ...data, settings: { ...data.settings, nginx_log: e.target.value } })}
            />
          </div>
        </div>
        <div className="field">
          <label>Fail2ban 日志</label>
          <input
            value={data.settings.fail2ban_log || ""}
            onChange={(e) => setData({ ...data, settings: { ...data.settings, fail2ban_log: e.target.value } })}
          />
        </div>
        <div className="form-actions">
          <button className="btn btn-primary" style={{ width: "auto" }}>
            保存设置
          </button>
        </div>
      </form>

      <form className="card panel" onSubmit={onPass}>
        <div className="panel-head">
          <div className="kicker">修改密码</div>
        </div>
        <div className="row-2">
          <div className="field">
            <label>旧密码</label>
            <input type="password" value={oldP} onChange={(e) => setOldP(e.target.value)} required />
          </div>
          <div className="field">
            <label>新密码</label>
            <input type="password" value={newP} onChange={(e) => setNewP(e.target.value)} required minLength={6} />
          </div>
        </div>
        <div className="form-actions">
          <button className="btn btn-primary" style={{ width: "auto" }}>
            更新密码
          </button>
        </div>
      </form>
    </div>
  );
}
