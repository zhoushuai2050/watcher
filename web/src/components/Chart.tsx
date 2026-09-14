import { useState } from "react";

type Series = { name: string; color: string; values: { t: number; v: number }[] };
type Hit = { name: string; color: string; t: number; v: number; x: number; y: number };

function fmtClock(t: number) {
  const d = new Date(t * 1000);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function fmtAxis(t: number) {
  const d = new Date(t * 1000);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

const HIT_R = 14;

export default function Chart({
  series,
  height = 228,
  format = (n: number) => String(n),
}: {
  series: Series[];
  height?: number;
  format?: (n: number) => string;
}) {
  const width = 760;
  const pad = { top: 18, right: 18, bottom: 30, left: 56 };
  const [hit, setHit] = useState<Hit | null>(null);

  const points = series.flatMap((s) => s.values);
  if (points.length === 0) {
    return <div className="empty">还没有足够的采样，等采集跑一会儿。</div>;
  }

  const times = points.map((p) => p.t);
  const minT = Math.min(...times);
  const maxT = Math.max(...times);
  const rawMax = Math.max(...points.map((p) => p.v), 1);
  const minV = 0;
  const maxV = rawMax * 1.08;
  const innerW = width - pad.left - pad.right;
  const innerH = height - pad.top - pad.bottom;
  const x = (t: number) => pad.left + (maxT === minT ? innerW / 2 : ((t - minT) / (maxT - minT)) * innerW);
  const y = (v: number) => pad.top + ((maxV - v) / (maxV - minV || 1)) * innerH;
  const ticks = [minV, maxV / 2, maxV];
  const xTicks = [minT, Math.round((minT + maxT) / 2), maxT];
  const gid = series.map((s) => s.name.replace(/\W+/g, "")).join("-") || "g";

  function pathOf(values: { t: number; v: number }[]) {
    return values.map((p, i) => `${i === 0 ? "M" : "L"} ${x(p.t).toFixed(1)} ${y(p.v).toFixed(1)}`).join(" ");
  }
  function areaOf(values: { t: number; v: number }[]) {
    if (!values.length) return "";
    const base = pad.top + innerH;
    return `${pathOf(values)} L ${x(values[values.length - 1].t).toFixed(1)} ${base} L ${x(values[0].t).toFixed(1)} ${base} Z`;
  }

  function hitFromClient(svg: SVGSVGElement, clientX: number, clientY: number): Hit | null {
    const rect = svg.getBoundingClientRect();
    const px = ((clientX - rect.left) / rect.width) * width;
    const py = ((clientY - rect.top) / rect.height) * height;
    let best: Hit | null = null;
    let dist = HIT_R;
    for (const s of series) {
      for (const p of s.values) {
        const hx = x(p.t);
        const hy = y(p.v);
        const d = Math.hypot(hx - px, hy - py);
        if (d <= dist) {
          dist = d;
          best = { name: s.name, color: s.color, t: p.t, v: p.v, x: hx, y: hy };
        }
      }
    }
    return best;
  }

  const tipLeftPct = hit ? (hit.x / width) * 100 : 0;
  const tipTopPct = hit ? (hit.y / height) * 100 : 0;

  return (
    <div className={`chart-wrap ${hit ? "is-hot" : ""}`}>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        onMouseMove={(e) => setHit(hitFromClient(e.currentTarget, e.clientX, e.clientY))}
        onMouseLeave={() => setHit(null)}
        onTouchStart={(e) => setHit(hitFromClient(e.currentTarget, e.touches[0].clientX, e.touches[0].clientY))}
        onTouchMove={(e) => setHit(hitFromClient(e.currentTarget, e.touches[0].clientX, e.touches[0].clientY))}
      >
        <defs>
          {series.map((s, i) => (
            <linearGradient key={s.name} id={`${gid}-${i}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={s.color} stopOpacity="0.22" />
              <stop offset="100%" stopColor={s.color} stopOpacity="0" />
            </linearGradient>
          ))}
        </defs>
        {ticks.map((tick) => (
          <g key={tick}>
            <line x1={pad.left} x2={width - pad.right} y1={y(tick)} y2={y(tick)} stroke="rgba(140,170,190,0.1)" />
            <text x={8} y={y(tick) + 4} fontSize="11" fill="#7d8c9c">
              {format(tick)}
            </text>
          </g>
        ))}
        {series.map((s, i) => (
          <g key={s.name}>
            <path d={areaOf(s.values)} fill={`url(#${gid}-${i})`} />
            <path d={pathOf(s.values)} fill="none" stroke={s.color} strokeWidth="1.15" strokeLinejoin="round" strokeLinecap="round" />
          </g>
        ))}
        {hit ? <circle cx={hit.x} cy={hit.y} r="3.4" fill={hit.color} stroke="#0b1014" strokeWidth="1.2" /> : null}
        {xTicks.map((t) => (
          <text key={t} x={x(t)} y={height - 8} textAnchor="middle" fontSize="11" fill="#7d8c9c">
            {fmtAxis(t)}
          </text>
        ))}
      </svg>
      {hit ? (
        <div
          className={`chart-tip ${tipLeftPct > 62 ? "is-left" : ""}`}
          style={{ left: `${tipLeftPct}%`, top: `${tipTopPct}%` }}
        >
          <time>{fmtClock(hit.t)}</time>
          <div className="chart-tip-row">
            <span>
              <i style={{ background: hit.color }} />
              {hit.name}
            </span>
            <b>{format(hit.v)}</b>
          </div>
        </div>
      ) : null}
      <div className="legend">
        {series.map((s) => (
          <span key={s.name}>
            <i style={{ background: s.color }} />
            {s.name}
          </span>
        ))}
      </div>
    </div>
  );
}

export function Sparkline({ values, color = "#3ee0b2" }: { values: number[]; color?: string }) {
  if (!values.length) return <div className="spark" />;
  const w = 160;
  const h = 42;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const pts = values.map((v, i) => {
    const x = values.length === 1 ? w / 2 : (i / (values.length - 1)) * w;
    const y = max === min ? h / 2 : h - ((v - min) / (max - min)) * (h - 6) - 3;
    return { x, y };
  });
  const line = pts.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(" ");
  const area = `${line} L ${pts[pts.length - 1].x.toFixed(1)} ${h} L ${pts[0].x.toFixed(1)} ${h} Z`;
  return (
    <svg className="spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <path d={area} fill={color} opacity="0.16" />
      <path d={line} fill="none" stroke={color} strokeWidth="1.15" strokeLinejoin="round" />
    </svg>
  );
}
