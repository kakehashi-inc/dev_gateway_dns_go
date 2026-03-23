import { useEffect } from "react";
import { Routes, Route, NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";
import Dashboard from "./pages/Dashboard";
import ProxySettings from "./pages/ProxySettings";
import DNSManagement from "./pages/DNSManagement";
import CertManagement from "./pages/CertManagement";
import StatusMonitor from "./pages/StatusMonitor";
import SystemSettings from "./pages/SystemSettings";

const navItems = [
  { path: "/", labelKey: "nav.dashboard" },
  { path: "/proxy", labelKey: "nav.proxy" },
  { path: "/dns", labelKey: "nav.dns" },
  { path: "/certs", labelKey: "nav.certs" },
  { path: "/status", labelKey: "nav.status" },
  { path: "/settings", labelKey: "nav.settings" },
];

export default function App() {
  const { t, i18n } = useTranslation();

  useEffect(() => {
    document.documentElement.lang = i18n.language;
  }, [i18n.language]);

  const toggleLang = () => {
    i18n.changeLanguage(i18n.language === "ja" ? "en" : "ja");
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100">
      <header className="bg-white dark:bg-gray-800 shadow">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
          <h1 className="text-xl font-bold">DevGatewayDNS</h1>
          <button
            onClick={toggleLang}
            className="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-gray-700"
          >
            {i18n.language === "ja" ? "EN" : "JA"}
          </button>
        </div>
      </header>
      <div className="max-w-7xl mx-auto flex">
        <nav className="w-48 shrink-0 py-4 pr-4">
          <ul className="space-y-1">
            {navItems.map((item) => (
              <li key={item.path}>
                <NavLink
                  to={item.path}
                  end={item.path === "/"}
                  className={({ isActive }) =>
                    `block px-3 py-2 rounded text-sm ${isActive ? "bg-blue-600 text-white" : "hover:bg-gray-200 dark:hover:bg-gray-700"}`
                  }
                >
                  {t(item.labelKey)}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
        <main className="flex-1 py-4">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/proxy" element={<ProxySettings />} />
            <Route path="/dns" element={<DNSManagement />} />
            <Route path="/certs" element={<CertManagement />} />
            <Route path="/status" element={<StatusMonitor />} />
            <Route path="/settings" element={<SystemSettings />} />
          </Routes>
        </main>
      </div>
    </div>
  );
}
