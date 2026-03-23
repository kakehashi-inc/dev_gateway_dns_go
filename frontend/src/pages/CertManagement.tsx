import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useApi, apiPost } from "../hooks/useApi";

interface CertInfo {
  hostname: string;
  expires_at: string;
  created_at: string;
}

export default function CertManagement() {
  const { t } = useTranslation();
  const { data: certs, refetch } = useApi<CertInfo[]>("/certs");
  const [showQR, setShowQR] = useState(false);

  const regenerate = async (hostname: string) => {
    await apiPost(`/certs/${hostname}/regenerate`, {});
    refetch();
  };

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold">{t("certs.title")}</h2>

      <div className="bg-white dark:bg-gray-800 rounded p-4 shadow space-y-3">
        <h3 className="font-semibold">{t("certs.ca")}</h3>
        <div className="flex gap-2 flex-wrap">
          <a
            href="/api/v1/certs/ca/download?format=pem"
            className="px-3 py-1 border rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
          >
            {t("certs.downloadPEM")}
          </a>
          <a
            href="/api/v1/certs/ca/download?format=der"
            className="px-3 py-1 border rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
          >
            {t("certs.downloadDER")}
          </a>
          <a
            href="/api/v1/certs/ca/download?format=p12"
            className="px-3 py-1 border rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
          >
            {t("certs.downloadP12")}
          </a>
          <button
            onClick={() => setShowQR(!showQR)}
            className="px-3 py-1 border rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
          >
            {t("certs.qrcode")}
          </button>
        </div>
        {showQR && (
          <div className="mt-2">
            <img src="/api/v1/certs/ca/qrcode" alt="CA QR Code" className="w-64 h-64" />
          </div>
        )}
        <div className="mt-2 space-y-2 text-sm">
          <h4 className="font-semibold">{t("certs.installGuide")}</h4>
          <p>
            <strong>iOS:</strong> {t("certs.guide.ios")}
          </p>
          <p>
            <strong>Android:</strong> {t("certs.guide.android")}
          </p>
          <p>
            <strong>Windows:</strong> {t("certs.guide.windows")}
          </p>
          <p>
            <strong>macOS:</strong> {t("certs.guide.macos")}
          </p>
        </div>
      </div>

      <table className="w-full text-sm bg-white dark:bg-gray-800 rounded shadow">
        <thead>
          <tr className="border-b dark:border-gray-700 text-left">
            <th className="p-2">{t("certs.hostname")}</th>
            <th className="p-2">{t("certs.expires")}</th>
            <th className="p-2">{t("certs.created")}</th>
            <th className="p-2">{t("proxy.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {certs?.map((cert) => (
            <tr key={cert.hostname} className="border-t dark:border-gray-700">
              <td className="p-2 font-mono">{cert.hostname}</td>
              <td className="p-2">{new Date(cert.expires_at).toLocaleDateString()}</td>
              <td className="p-2">{new Date(cert.created_at).toLocaleDateString()}</td>
              <td className="p-2">
                <button onClick={() => regenerate(cert.hostname)} className="text-blue-600 text-xs hover:underline">
                  {t("certs.regenerate")}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
