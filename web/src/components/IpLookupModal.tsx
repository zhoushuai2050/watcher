import { useEffect, useState } from "react";
import { api } from "../lib/api";
import type { IPLookup } from "../lib/types";

export default function IpLookupModal({ ip, onClose }: { ip: string; onClose: () => void }) {
  const [data, setData] = useState<IPLookup | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let stop = false;
    setLoading(true);
    setError("");
    setData(null);
    void api<IPLookup>(`/api/v1/network/ip?ip=${encodeURIComponent(ip)}`)
      .then((next) => {
        if (!stop) setData(next);
      })
      .catch((err) => {
        if (!stop) setError(err instanceof Error ? err.message : "查询失败");
      })
      .finally(() => {
        if (!stop) setLoading(false);
      });
    return () => {
      stop = true;
    };
  }, [ip]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const rows: [string, string][] = data
    ? [
        ["地址", data.location || "—"],
        ["国家 / 地区", [data.country, data.region].filter(Boolean).join(" · ") || "—"],
        ["城市", data.city || "—"],
        ["运营商", data.isp || "—"],
        ["组织", data.org || "—"],
        ["ASN", data.asn || "—"],
        ["时区", data.timezone || "—"],
        ["坐标", data.lat || data.lon ? `${data.lat}, ${data.lon}` : "—"],
      ]
    : [];

  return (
    <div className="modal-backdrop" onClick={onClose} role="presentation">
      <div
        className="modal card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="ip-lookup-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="panel-head">
          <div>
            <div className="kicker">IP 归属</div>
            <h3 id="ip-lookup-title" className="ip" style={{ marginTop: 4 }}>
              {ip}
            </h3>
          </div>
          <button className="btn btn-ghost" onClick={onClose}>
            关闭
          </button>
        </div>
        {loading ? <p className="muted">正在查询…</p> : null}
        {error ? <div className="error">{error}</div> : null}
        {data?.private ? <p className="muted">{data.location}</p> : null}
        {data && !data.private ? (
          <dl className="kv">
            {rows.map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
          </dl>
        ) : null}
      </div>
    </div>
  );
}
