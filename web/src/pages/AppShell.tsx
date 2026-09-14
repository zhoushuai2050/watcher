import { NavLink, Outlet } from "react-router-dom";
import BrandMark from "../components/BrandMark";
import { IconBell, IconChart, IconGear, IconNet, IconOverview, IconProc, IconShield } from "../components/Icons";
import { useAuth } from "../lib/auth";

const links = [
  { to: "/", label: "总览", end: true, icon: <IconOverview /> },
  { to: "/resources", label: "资源", icon: <IconChart /> },
  { to: "/processes", label: "进程", icon: <IconProc /> },
  { to: "/network", label: "网络", icon: <IconNet /> },
  { to: "/security", label: "安全", icon: <IconShield /> },
  { to: "/alerts", label: "告警", icon: <IconBell /> },
  { to: "/settings", label: "设置", icon: <IconGear /> },
];

export default function AppShell() {
  const { user, logout } = useAuth();
  return (
    <div className="app-shell">
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
            <NavLink key={link.to} to={link.to} end={link.end} className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}>
              {link.icon}
              {link.label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <div className="who">{user?.name || "admin"}</div>
          <button className="btn btn-ghost logout" onClick={logout}>
            退出登录
          </button>
        </div>
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
      </nav>
    </div>
  );
}
