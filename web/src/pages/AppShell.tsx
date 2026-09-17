import { useEffect, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import BrandMark from "../components/BrandMark";
import { IconChart, IconGear, IconNet, IconOverview, IconShield } from "../components/Icons";
import { useAuth } from "../lib/auth";
import { useTheme } from "../lib/theme";

const SIDEBAR_KEY = "watcher.sidebar";

const links = [
  { to: "/", label: "总览", end: true, icon: <IconOverview /> },
  { to: "/resources", label: "资源", icon: <IconChart /> },
  { to: "/network", label: "网络", icon: <IconNet /> },
  { to: "/security", label: "安全", icon: <IconShield /> },
  { to: "/settings", label: "设置", icon: <IconGear /> },
];

export default function AppShell() {
  const { user, logout } = useAuth();
  const { theme, toggle } = useTheme();
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(SIDEBAR_KEY) === "collapsed";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(SIDEBAR_KEY, collapsed ? "collapsed" : "expanded");
    } catch {
      /* ignore */
    }
  }, [collapsed]);

  return (
    <div className={`app-shell${collapsed ? " is-collapsed" : ""}`}>
      <aside className="sidebar">
        <div className="brand-row">
          <BrandMark />
          <div className="brand-copy">
            <p className="eyebrow">Watcher</p>
            <strong>本机监控</strong>
          </div>
        </div>
        <nav className="side-nav">
          {links.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              end={link.end}
              title={link.label}
              className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}
            >
              {link.icon}
              <span className="nav-label">{link.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <div className="who">{user?.name || "admin"}</div>
          <button className="btn btn-ghost theme-toggle" type="button" onClick={toggle} title="theme">
            {collapsed ? (
              theme === "light" ? "light" : "dark"
            ) : (
              <>
                <span className={theme === "light" ? "" : "muted"}>light</span>
                <span className="muted"> / </span>
                <span className={theme === "dark" ? "" : "muted"}>dark</span>
              </>
            )}
          </button>
          <button className="btn btn-ghost logout" onClick={logout} title="退出登录">
            {collapsed ? "退出" : "退出登录"}
          </button>
        </div>
        <button
          className="sidebar-fold"
          type="button"
          aria-label={collapsed ? "展开侧栏" : "折叠侧栏"}
          title={collapsed ? "展开" : "折叠"}
          onClick={() => setCollapsed((v) => !v)}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
            {collapsed ? <path d="M9 6l6 6-6 6" /> : <path d="M15 6l-6 6 6 6" />}
          </svg>
          <span className="fold-label">{collapsed ? "展开" : "折叠"}</span>
        </button>
      </aside>
      <main className="content">
        <Outlet />
      </main>
      <nav className="bottom-nav">
        {links.map((link) => (
          <NavLink key={link.to} to={link.to} end={link.end} className={({ isActive }) => (isActive ? "active" : "")}>
            {link.icon}
            {link.label}
          </NavLink>
        ))}
        <button type="button" onClick={toggle}>
          {theme === "light" ? "light" : "dark"}
        </button>
      </nav>
    </div>
  );
}
