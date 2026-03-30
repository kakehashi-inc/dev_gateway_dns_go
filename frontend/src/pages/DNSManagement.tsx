import { useState } from "react";
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

const recordTypes = ["A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "PTR", "CAA", "SOA", "NAPTR", "SSHFP", "TLSA", "DS", "DNSKEY"];

const hintClass = "text-gray-500 dark:text-gray-400 text-xs mt-1";

export default function DNSManagement() {
  const { t } = useTranslation();
  const { data: records, refetch } = useApi<DNSRecord[]>("/dns/records");
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", type: "A", value: "", ttl: 300 });

  const addRecord = async () => {
    await apiPost("/dns/records", form);
    setShowForm(false);
    setForm({ name: "", type: "A", value: "", ttl: 300 });
    refetch();
  };

  const cancelForm = () => {
    setShowForm(false);
    setForm({ name: "", type: "A", value: "", ttl: 300 });
  };

  const deleteRecord = async (id: number) => {
    await apiDelete(`/dns/records/${id}`);
    refetch();
  };

  const valuePlaceholder = (type: string): string => {
    const key = `dns.valuePlaceholder.${type}`;
    const val = t(key);
    return val !== key ? val : t("dns.valuePlaceholder.default");
  };

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

      <p className="text-sm text-gray-600 dark:text-gray-400">{t("dns.description")}</p>

      {showForm && (
        <div className="bg-white dark:bg-gray-800 rounded p-4 shadow space-y-4">
          <div className="grid grid-cols-4 gap-3">
            <div className="col-span-2">
              <label className="block text-sm font-medium mb-1">{t("dns.name")}</label>
              <input
                placeholder={t("dns.namePlaceholder")}
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                className="w-full border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
              />
              <p className={hintClass}>{t("dns.nameHint")}</p>
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">{t("dns.type")}</label>
              <select
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value })}
                className="w-full border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
              >
                {recordTypes.map((rt) => (
                  <option key={rt} value={rt}>
                    {rt}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">{t("dns.ttl")}</label>
              <input
                type="number"
                placeholder="300"
                value={form.ttl}
                onChange={(e) => setForm({ ...form, ttl: parseInt(e.target.value) || 300 })}
                className="w-full border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
              />
              <p className={hintClass}>{t("dns.ttlHint")}</p>
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">{t("dns.value")}</label>
            <input
              placeholder={valuePlaceholder(form.type)}
              value={form.value}
              onChange={(e) => setForm({ ...form, value: e.target.value })}
              className="w-full border rounded px-2 py-1 dark:bg-gray-700 dark:border-gray-600"
            />
            <p className={hintClass}>{t("dns.valueHint")}</p>
          </div>
          <div className="flex gap-2">
            <button onClick={addRecord} className="px-3 py-1 bg-blue-600 text-white rounded text-sm">
              {t("dns.save")}
            </button>
            <button onClick={cancelForm} className="px-3 py-1 border rounded text-sm">
              {t("dns.cancel")}
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
            <th className="p-2"></th>
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
                  className={`text-xs px-1.5 py-0.5 rounded ${rec.locked ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300" : "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300"}`}
                >
                  {rec.locked ? t("dns.locked") : t("dns.manual")}
                </span>
              </td>
              <td className="p-2">
                {!rec.locked && rec.id && (
                  <button onClick={() => deleteRecord(rec.id!)} className="text-red-600 text-xs hover:underline">
                    {t("dns.delete")}
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
