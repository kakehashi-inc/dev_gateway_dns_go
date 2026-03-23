import { useState, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

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

export default function StatusMonitor() {
  const { t } = useTranslation();
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/v1/status/live`);
    ws.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        if (Array.isArray(data)) setLogs(data);
      } catch {
        /* ignore */
      }
    };
    wsRef.current = ws;
    return () => ws.close();
  }, []);

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("status.title")}</h2>

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
        <h3 className="font-semibold mb-2">{t("status.accessLog")}</h3>
        <div className="max-h-96 overflow-y-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left border-b dark:border-gray-700">
                <th className="p-1">Time</th>
                <th className="p-1">Source</th>
                <th className="p-1">Client</th>
                <th className="p-1">Host</th>
                <th className="p-1">Method</th>
                <th className="p-1">Path</th>
                <th className="p-1">Status</th>
                <th className="p-1">Time(ms)</th>
                <th className="p-1">Backend</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log, i) => (
                <tr key={i} className="border-t dark:border-gray-700">
                  <td className="p-1 font-mono">{new Date(log.timestamp).toLocaleTimeString()}</td>
                  <td className="p-1">{log.source}</td>
                  <td className="p-1 font-mono">{log.client_ip}</td>
                  <td className="p-1 font-mono">{log.hostname}</td>
                  <td className="p-1">{log.method}</td>
                  <td className="p-1 font-mono truncate max-w-32">{log.path}</td>
                  <td className={`p-1 ${log.status_code >= 400 ? "text-red-600" : "text-green-600"}`}>
                    {log.status_code}
                  </td>
                  <td className="p-1">{log.response_time_ms}</td>
                  <td className="p-1 font-mono">{log.backend}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
