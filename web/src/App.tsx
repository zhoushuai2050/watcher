import type { ReactNode } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import AlertsPage from "./pages/AlertsPage";
import AppShell from "./pages/AppShell";
import LoginPage from "./pages/LoginPage";
import NetworkPage from "./pages/NetworkPage";
import OverviewPage from "./pages/OverviewPage";
import ProcessDetailPage from "./pages/ProcessDetailPage";
import ProcessesPage from "./pages/ProcessesPage";
import ResourcesPage from "./pages/ResourcesPage";
import SecurityPage from "./pages/SecurityPage";
import SettingsPage from "./pages/SettingsPage";

function Guard({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="auth-panel">加载中...</div>;
  if (!user) return <Navigate to="/login" replace />;
  return children;
}

function Guest({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="auth-panel">加载中...</div>;
  if (user) return <Navigate to="/" replace />;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <Guest>
            <LoginPage />
          </Guest>
        }
      />
      <Route
        path="/"
        element={
          <Guard>
            <AppShell />
          </Guard>
        }
      >
        <Route index element={<OverviewPage />} />
        <Route path="resources" element={<ResourcesPage />} />
        <Route path="processes" element={<ProcessesPage />} />
        <Route path="processes/view" element={<ProcessDetailPage />} />
        <Route path="network" element={<NetworkPage />} />
        <Route path="security" element={<SecurityPage />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
