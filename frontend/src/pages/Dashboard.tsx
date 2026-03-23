import { useState, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useApi } from "../hooks/useApi";

interface Overview {
  version: string;
  uptime: string;
  active_rules: number;
}
interface NIC {
  name: string;
  ips: string[];
}
interface PortHealth {
  service: string;
  port: number;
  protocol: string;
  bound: boolean;
  loopback: boolean;
}
interface AccessLog {
  timestamp: string;
  source: string;
  client_ip: string;
  hostname: string;
  method: string;
  path: string;
  status_code: number;
  response_time_ms: number;
  backend: string;
}

export default function Dashboard() {
  const { t } = useTranslation();
  const overview = useApi<Overview>("/status/overview");
  const nics = useApi<NIC[]>("/status/interfaces");
  const health = useApi<PortHealth[]>("/status/health");
  const [recentLogs, setRecentLogs] = useState<AccessLog[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/v1/status/live`);
    ws.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        if (Array.isArray(data)) setRecentLogs(data);
      } catch {
        /* ignore */
      }
    };
    wsRef.current = ws;
    return () => ws.close();
  }, []);

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold">{t("dashboard.title")}</h2>

      {overview.data && (
        <div className="grid grid-cols-3 gap-4">
          <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
            <div className="text-sm text-gray-500">{t("dashboard.version")}</div>
            <div className="text-2xl font-bold">{overview.data.version}</div>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
            <div className="text-sm text-gray-500">{t("dashboard.uptime")}</div>
            <div className="text-2xl font-bold">{overview.data.uptime}</div>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
            <div className="text-sm text-gray-500">{t("dashboard.activeRules")}</div>
            <div className="text-2xl font-bold">{overview.data.active_rules}</div>
          </div>
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
        <div className="flex items-center justify-between mb-2">
          <h3 className="font-semibold">{t("dashboard.health")}</h3>
          <button
            onClick={health.refetch}
            className="text-sm px-2 py-1 border rounded hover:bg-gray-100 dark:hover:bg-gray-700"
          >
            {t("dashboard.recheck")}
          </button>
        </div>
        <table className="w-full text-sm">
          <tbody>
            {health.data?.map((h, i) => (
              <tr key={i} className="border-t dark:border-gray-700">
                <td className="py-1">{h.service}</td>
                <td>
                  :{h.port}/{h.protocol}
                </td>
                <td>{h.bound ? "\u2705" : "\u274C"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
        <h3 className="font-semibold mb-2">{t("dashboard.interfaces")}</h3>
        <table className="w-full text-sm">
          <tbody>
            {nics.data?.map((nic) => (
              <tr key={nic.name} className="border-t dark:border-gray-700">
                <td className="py-1 font-mono">{nic.name}</td>
                <td>{nic.ips.join(", ")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
        <h3 className="font-semibold mb-2">{t("dashboard.recentLogs")}</h3>
        <div className="max-h-48 overflow-y-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left border-b dark:border-gray-700">
                <th className="p-1">Time</th>
                <th className="p-1">Source</th>
                <th className="p-1">Host</th>
                <th className="p-1">Method</th>
                <th className="p-1">Path</th>
                <th className="p-1">Status</th>
              </tr>
            </thead>
            <tbody>
              {recentLogs.slice(0, 10).map((log, i) => (
                <tr key={i} className="border-t dark:border-gray-700">
                  <td className="p-1 font-mono">{new Date(log.timestamp).toLocaleTimeString()}</td>
                  <td className="p-1">{log.source}</td>
                  <td className="p-1 font-mono">{log.hostname}</td>
                  <td className="p-1">{log.method}</td>
                  <td className="p-1 font-mono truncate max-w-24">{log.path}</td>
                  <td className={`p-1 ${log.status_code >= 400 ? "text-red-600" : "text-green-600"}`}>
                    {log.status_code}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
