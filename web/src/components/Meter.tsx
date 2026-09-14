export default function Meter({ value }: { value: number }) {
  const pct = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const tone = pct >= 90 ? "hot" : pct >= 75 ? "warn" : "ok";
  return (
    <div className={`meter meter-${tone}`} title={`${pct.toFixed(0)}%`}>
      <span style={{ width: `${pct}%` }} />
    </div>
  );
}
