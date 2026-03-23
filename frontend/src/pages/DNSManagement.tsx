import { useState, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useApi, apiPost, apiDelete } from "../hooks/useApi";

interface DNSRecord {
  id?: number;
  name: string;
  type: string;
  value: string;
  ttl: number;
  locked: boolean;
  source: string;
}

interface QueryLog {
  client_ip: string;
  hostname: string;
  record_type: string;
  response_type: string;
  response_time_ns: number;
  timestamp: string;
}

export default function DNSManagement() {
  const { t } = useTranslation();
  const { data: records, refetch } = useApi<DNSRecord[]>("/dns/records");
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", type: "A", value: "", ttl: 300 });
  const [queryLogs, setQueryLogs] = useState<QueryLog[]>([]);
  const [filterHostname, setFilterHostname] = useState("");
  const [filterClientIP, setFilterClientIP] = useState("");
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/v1/dns/queries/live`);
    ws.onmessage = (e) => {
      try {
        setQueryLogs(JSON.parse(e.data));
      } catch {
        /* ignore parse errors */
      }
    };
    wsRef.current = ws;
    return () => ws.close();
  }, []);

  const addRecord = async () => {
    await apiPost("/dns/records", form);
    setShowForm(false);
    setForm({ name: "", type: "A", value: "", ttl: 300 });
    refetch();
  };

  const deleteRecord = async (id: number) => {
    await apiDelete(`/dns/records/${id}`);
    refetch();
  };

  const filteredLogs = queryLogs.filter((log) => {
    if (filterHostname && !log.hostname.includes(filterHostname)) return false;
    if (filterClientIP && !log.client_ip.includes(filterClientIP)) return false;
    return true;
  });

  const recordTypes = ["A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "PTR", "CAA", "SOA"];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("dns.title")}</h2>
        <button
          onClick={() => setShowForm(true)}
          className="px-3 py-1 bg-blue-600 text-white rounded text-sm hover:bg-blue-700"
        >
          {t("dns.addRecord")}
        </button>
      </div>

      {showForm && (
        <div className="bg-white dark:bg-gray-800 rounded p-4 shadow space-y-3">
          <div className="flex gap-2">
            <input
              placeholder={t("dns.name")}
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
            <select
              value={form.type}
              onChange={(e) => setForm({ ...form, type: e.target.value })}
              className="border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            >
              {recordTypes.map((rt) => (
                <option key={rt} value={rt}>
                  {rt}
                </option>
              ))}
            </select>
          </div>
          <div className="flex gap-2">
            <input
              placeholder={t("dns.value")}
              value={form.value}
              onChange={(e) => setForm({ ...form, value: e.target.value })}
              className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
            <input
              type="number"
              placeholder="TTL"
              value={form.ttl}
              onChange={(e) => setForm({ ...form, ttl: parseInt(e.target.value) || 300 })}
              className="w-20 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
          </div>
          <div className="flex gap-2">
            <button onClick={addRecord} className="px-3 py-1 bg-blue-600 text-white rounded text-sm">
              {t("proxy.save")}
            </button>
            <button onClick={() => setShowForm(false)} className="px-3 py-1 border rounded text-sm">
              {t("proxy.cancel")}
            </button>
          </div>
        </div>
      )}

      <table className="w-full text-sm bg-white dark:bg-gray-800 rounded shadow">
        <thead>
          <tr className="border-b dark:border-gray-700 text-left">
            <th className="p-2">{t("dns.name")}</th>
            <th className="p-2">{t("dns.type")}</th>
            <th className="p-2">{t("dns.value")}</th>
            <th className="p-2">{t("dns.ttl")}</th>
            <th className="p-2">{t("dns.source")}</th>
            <th className="p-2">{t("proxy.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {records?.map((rec, i) => (
            <tr key={rec.id ?? `auto-${i}`} className="border-t dark:border-gray-700">
              <td className="p-2 font-mono">{rec.name}</td>
              <td className="p-2">{rec.type}</td>
              <td className="p-2 font-mono text-xs">{rec.value}</td>
              <td className="p-2">{rec.ttl}</td>
              <td className="p-2">
                <span
                  className={`text-xs px-1 rounded ${rec.locked ? "bg-yellow-100 text-yellow-800" : "bg-blue-100 text-blue-800"}`}
                >
                  {rec.locked ? t("dns.locked") : t("dns.manual")}
                </span>
              </td>
              <td className="p-2">
                {!rec.locked && rec.id && (
                  <button onClick={() => deleteRecord(rec.id!)} className="text-red-600 text-xs hover:underline">
                    {t("proxy.delete")}
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

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
                <th className="p-1">Client IP</th>
                <th className="p-1">Hostname</th>
                <th className="p-1">Type</th>
                <th className="p-1">Response</th>
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
