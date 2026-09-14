import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import BrandMark from "../components/BrandMark";
import { useAuth } from "../lib/auth";

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [name, setName] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setPending(true);
    setError("");
    try {
      await login(name, password);
      navigate("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="auth-layout">
      <section className="auth-story">
        <div>
          <BrandMark />
          <p className="eyebrow" style={{ marginTop: 28 }}>
            Watcher
          </p>
          <h1>看住这台机器。</h1>
          <p className="lead">资源、进程、端口，还有外面谁在扫、谁在爆破。第一期只盯本机。</p>
        </div>
        <p className="muted">单机自监控 · SSH / Web / Fail2ban / 连接异常</p>
      </section>
      <section className="auth-panel">
        <form className="card auth-card" onSubmit={onSubmit}>
          <p className="eyebrow">本机面板</p>
          <h2>登录</h2>
          <p className="hint">没有公开注册。账号写在配置里。</p>
          {error ? <div className="error">{error}</div> : null}
          <div className="field">
            <label>用户名</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required autoComplete="username" />
          </div>
          <div className="field">
            <label>密码</label>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" />
          </div>
          <button className="btn btn-primary" disabled={pending}>
            {pending ? "登录中…" : "进入监控"}
          </button>
        </form>
      </section>
    </div>
  );
}
