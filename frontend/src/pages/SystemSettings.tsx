import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useApi, apiPut } from "../hooks/useApi";

interface Settings {
  http_port: number;
  https_port: number;
  dns_port: number;
  proxy_port: number;
  admin_port: number;
  listen_addresses: string[];
  upstream_dns_fallback: string[];
  dns_query_history_size: number;
  log_level: string;
  access_log_retention_days: number;
}

export default function SystemSettings() {
  const { t } = useTranslation();
  const { data, refetch } = useApi<Settings>("/settings");
  const [form, setForm] = useState<Settings | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (data) setForm({ ...data });
  }, [data]);

  const save = async () => {
    if (!form) return;
    await apiPut("/settings", form);
    setSaved(true);
    setTimeout(() => setSaved(false), 3000);
    refetch();
  };

  if (!form) return <p>{t("common.loading")}</p>;

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("settings.title")}</h2>

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow space-y-3 max-w-lg">
        {(
          [
            ["httpPort", "http_port", t("settings.httpPort")],
            ["httpsPort", "https_port", t("settings.httpsPort")],
            ["dnsPort", "dns_port", t("settings.dnsPort")],
            ["proxyPort", "proxy_port", t("settings.proxyPort")],
            ["adminPort", "admin_port", t("settings.adminPort")],
          ] as const
        ).map(([, key, label]) => (
          <div key={key} className="flex items-center gap-2">
            <label className="w-48 text-sm">{label}</label>
            <input
              type="number"
              value={form[key as keyof Settings] as number}
              onChange={(e) => setForm({ ...form, [key]: parseInt(e.target.value) || 0 })}
              className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
          </div>
        ))}

        <div className="flex items-center gap-2">
          <label className="w-48 text-sm">{t("settings.listenAddresses")}</label>
          <input
            value={form.listen_addresses.join(",")}
            onChange={(e) => setForm({ ...form, listen_addresses: e.target.value.split(",").map((s) => s.trim()) })}
            className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
          />
        </div>

        <div className="flex items-center gap-2">
          <label className="w-48 text-sm">{t("settings.upstreamDns")}</label>
          <input
            value={form.upstream_dns_fallback.join(",")}
            onChange={(e) =>
              setForm({ ...form, upstream_dns_fallback: e.target.value.split(",").map((s) => s.trim()) })
            }
            className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
          />
        </div>

        <div className="flex items-center gap-2">
          <label className="w-48 text-sm">{t("settings.queryHistorySize")}</label>
          <input
            type="number"
            value={form.dns_query_history_size}
            onChange={(e) => setForm({ ...form, dns_query_history_size: parseInt(e.target.value) || 1000 })}
            className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
          />
        </div>

        <div className="flex items-center gap-2">
          <label className="w-48 text-sm">{t("settings.accessLogRetentionDays")}</label>
          <input
            type="number"
            min="1"
            value={form.access_log_retention_days}
            onChange={(e) => setForm({ ...form, access_log_retention_days: parseInt(e.target.value) || 7 })}
            className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
          />
        </div>

        <div className="flex items-center gap-2">
          <label className="w-48 text-sm">{t("settings.logLevel")}</label>
          <select
            value={form.log_level}
            onChange={(e) => setForm({ ...form, log_level: e.target.value })}
            className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
          >
            <option value="debug">debug</option>
            <option value="info">info</option>
            <option value="warn">warn</option>
            <option value="error">error</option>
          </select>
        </div>

        <div className="flex items-center gap-2">
          <button onClick={save} className="px-4 py-1 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
            {t("settings.save")}
          </button>
          {saved && <span className="text-green-600 text-sm">{t("settings.saved")}</span>}
        </div>
      </div>
    </div>
  );
}
