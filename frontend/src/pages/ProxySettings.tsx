import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useApi, apiPost, apiDelete, apiPatch } from "../hooks/useApi";

interface ProxyRule {
  id: number;
  hostname: string;
  backend_protocol: string;
  backend_ip: string | null;
  backend_port: number;
  enabled: boolean;
}

export default function ProxySettings() {
  const { t } = useTranslation();
  const { data: rules, refetch } = useApi<ProxyRule[]>("/proxy/rules");
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ hostname: "", backend_protocol: "http", backend_ip: "", backend_port: 8080 });

  const addRule = async () => {
    await apiPost("/proxy/rules", {
      ...form,
      backend_ip: form.backend_ip || null,
      enabled: true,
    });
    setShowForm(false);
    setForm({ hostname: "", backend_protocol: "http", backend_ip: "", backend_port: 8080 });
    refetch();
  };

  const deleteRule = async (id: number) => {
    await apiDelete(`/proxy/rules/${id}`);
    refetch();
  };

  const toggleRule = async (id: number) => {
    await apiPatch(`/proxy/rules/${id}/toggle`);
    refetch();
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("proxy.title")}</h2>
        <button
          onClick={() => setShowForm(true)}
          className="px-3 py-1 bg-blue-600 text-white rounded text-sm hover:bg-blue-700"
        >
          {t("proxy.add")}
        </button>
      </div>

      {showForm && (
        <div className="bg-white dark:bg-gray-800 rounded p-4 shadow space-y-3">
          <input
            placeholder={t("proxy.hostname")}
            value={form.hostname}
            onChange={(e) => setForm({ ...form, hostname: e.target.value })}
            className="w-full border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
          />
          <div className="flex gap-2">
            <select
              value={form.backend_protocol}
              onChange={(e) => setForm({ ...form, backend_protocol: e.target.value })}
              className="border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            >
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
            </select>
            <input
              placeholder={t("proxy.ipAuto")}
              value={form.backend_ip}
              onChange={(e) => setForm({ ...form, backend_ip: e.target.value })}
              className="flex-1 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
            <input
              type="number"
              placeholder={t("proxy.port")}
              value={form.backend_port}
              onChange={(e) => setForm({ ...form, backend_port: parseInt(e.target.value) || 0 })}
              className="w-24 border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
          </div>
          <div className="flex gap-2">
            <button onClick={addRule} className="px-3 py-1 bg-blue-600 text-white rounded text-sm">
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
            <th className="p-2">{t("proxy.hostname")}</th>
            <th className="p-2">{t("proxy.protocol")}</th>
            <th className="p-2">{t("proxy.ip")}</th>
            <th className="p-2">{t("proxy.port")}</th>
            <th className="p-2">{t("proxy.enabled")}</th>
            <th className="p-2">{t("proxy.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {rules?.map((rule) => (
            <tr key={rule.id} className="border-t dark:border-gray-700">
              <td className="p-2 font-mono">{rule.hostname}</td>
              <td className="p-2">{rule.backend_protocol}</td>
              <td className="p-2">{rule.backend_ip || t("proxy.ipAuto")}</td>
              <td className="p-2">{rule.backend_port}</td>
              <td className="p-2">
                <button
                  onClick={() => toggleRule(rule.id)}
                  className={`px-2 py-0.5 rounded text-xs ${rule.enabled ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-600"}`}
                >
                  {rule.enabled ? "ON" : "OFF"}
                </button>
              </td>
              <td className="p-2">
                <button onClick={() => deleteRule(rule.id)} className="text-red-600 text-xs hover:underline">
                  {t("proxy.delete")}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
