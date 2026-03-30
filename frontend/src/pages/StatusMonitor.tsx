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

interface QueryLog {
  client_ip: string;
  hostname: string;
  record_type: string;
  response_type: string;
  response_time_ns: number;
  timestamp: string;
}

export default function StatusMonitor() {
  const { t } = useTranslation();
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [queryLogs, setQueryLogs] = useState<QueryLog[]>([]);
  const [filterHostname, setFilterHostname] = useState("");
  const [filterClientIP, setFilterClientIP] = useState("");
  const wsAccessRef = useRef<WebSocket | null>(null);
  const wsDnsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";

    const wsAccess = new WebSocket(`${proto}//${location.host}/api/v1/status/live`);
    wsAccess.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        if (Array.isArray(data)) setLogs(data);
      } catch {
        /* ignore */
      }
    };
    wsAccessRef.current = wsAccess;

    const wsDns = new WebSocket(`${proto}//${location.host}/api/v1/dns/queries/live`);
    wsDns.onmessage = (e) => {
      try {
        setQueryLogs(JSON.parse(e.data));
      } catch {
        /* ignore */
      }
    };
    wsDnsRef.current = wsDns;

    return () => {
      wsAccess.close();
      wsDns.close();
    };
  }, []);

  const filteredLogs = queryLogs.filter((log) => {
    if (filterHostname && !log.hostname.includes(filterHostname)) return false;
    if (filterClientIP && !log.client_ip.includes(filterClientIP)) return false;
    return true;
  });

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold">{t("status.title")}</h2>
      <p className="text-sm text-gray-600 dark:text-gray-400">{t("status.description")}</p>

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
        <h3 className="font-semibold mb-2">{t("status.accessLog")}</h3>
        <div className="max-h-96 overflow-y-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left border-b dark:border-gray-700">
                <th className="p-1">{t("status.time")}</th>
                <th className="p-1">{t("status.source")}</th>
                <th className="p-1">{t("status.client")}</th>
                <th className="p-1">{t("status.host")}</th>
                <th className="p-1">{t("status.method")}</th>
                <th className="p-1">{t("status.path")}</th>
                <th className="p-1">{t("status.statusCode")}</th>
                <th className="p-1">{t("status.responseTime")}</th>
                <th className="p-1">{t("status.backend")}</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log, i) => (
                <tr key={i} className="border-t dark:border-gray-700">
                  <td className="p-1 font-mono">{new Date(log.timestamp).toLocaleString()}</td>
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

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow">
        <h3 className="font-semibold mb-2">{t("dns.queryHistory")}</h3>
        <div className="flex gap-2 mb-2">
          <input
            placeholder={t("dns.filterHostname")}
            value={filterHostname}
            onChange={(e) => setFilterHostname(e.target.value)}
            className="flex-1 border rounded px-2 py-1 text-sm dark:bg-gray-700 dark:border-gray-600"
          />
          <input
            placeholder={t("dns.filterClientIP")}
            value={filterClientIP}
            onChange={(e) => setFilterClientIP(e.target.value)}
            className="flex-1 border rounded px-2 py-1 text-sm dark:bg-gray-700 dark:border-gray-600"
          />
        </div>
        <div className="max-h-64 overflow-y-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left border-b dark:border-gray-700">
                <th className="p-1">{t("dns.queryClientIP")}</th>
                <th className="p-1">{t("dns.queryHostname")}</th>
                <th className="p-1">{t("dns.queryType")}</th>
                <th className="p-1">{t("dns.queryResponse")}</th>
              </tr>
            </thead>
            <tbody>
              {filteredLogs
                .slice(-50)
                .reverse()
                .map((log, i) => (
                  <tr key={i} className="border-t dark:border-gray-700">
                    <td className="p-1 font-mono">{log.client_ip}</td>
                    <td className="p-1 font-mono">{log.hostname}</td>
                    <td className="p-1">{log.record_type}</td>
                    <td className="p-1">{log.response_type}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
