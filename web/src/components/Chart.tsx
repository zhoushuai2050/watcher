import { useEffect, useRef } from "react";
import * as echarts from "echarts/core";
import { LineChart } from "echarts/charts";
import { AxisPointerComponent, DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { EChartsCoreOption } from "echarts/core";
import { useTheme } from "../lib/theme";

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, AxisPointerComponent, CanvasRenderer]);

type Point = { t: number; v: number };
type Series = { name: string; color: string; values: Point[] };

function hexAlpha(hex: string, a: number) {
  const n = hex.replace("#", "");
  const r = parseInt(n.slice(0, 2), 16);
  const g = parseInt(n.slice(2, 4), 16);
  const b = parseInt(n.slice(4, 6), 16);
  return `rgba(${r},${g},${b},${a})`;
}

function fmtClock(t: number) {
  const d = new Date(t);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function fmtAxis(t: number, spanMs: number) {
  const d = new Date(t);
  const p = (n: number) => String(n).padStart(2, "0");
  if (spanMs >= 2 * 86400000) return `${p(d.getMonth() + 1)}-${p(d.getDate())}`;
  return `${p(d.getHours())}:${p(d.getMinutes())}`;
}

export default function Chart({
  series,
  height = 280,
  format = (n: number) => String(n),
  variant = "full",
}: {
  series: Series[];
  height?: number;
  format?: (n: number) => string;
  variant?: "full" | "spark";
}) {
  const elRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);
  const formatRef = useRef(format);
  formatRef.current = format;
  const { theme } = useTheme();
  const points = series.flatMap((s) => s.values);
  const empty = points.length === 0;

  useEffect(() => {
    const el = elRef.current;
    if (!el || empty) return;
    const chart = echarts.getInstanceByDom(el) || echarts.init(el, undefined, { renderer: "canvas" });
    chartRef.current = chart;
    const ro = new ResizeObserver(() => chart.resize());
    ro.observe(el);
    return () => {
      ro.disconnect();
      chart.dispose();
      chartRef.current = null;
    };
  }, [empty]);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || empty) return;
    const times = points.map((p) => p.t);
    const minT = Math.min(...times);
    const maxT = Math.max(...times);
    const spanMs = variant === "spark" ? 0 : (maxT - minT) * 1000;
    const fmt = (n: number) => formatRef.current(n);
    const css = getComputedStyle(document.documentElement);
    const ink = css.getPropertyValue("--ink").trim() || "#e8eef4";
    const muted = css.getPropertyValue("--muted").trim() || "#8393a4";
    const surface = css.getPropertyValue("--surface").trim() || "#121a22";
    const surface2 = css.getPropertyValue("--surface-2").trim() || "#161f28";
    const accent = css.getPropertyValue("--accent").trim() || "#3ee0b2";
    const line = css.getPropertyValue("--line").trim() || "rgba(148,175,196,0.12)";
    const accentDim = css.getPropertyValue("--accent-dim").trim() || "rgba(62,224,178,0.14)";

    const option: EChartsCoreOption =
      variant === "spark"
        ? {
            animation: false,
            grid: { left: 0, right: 0, top: 4, bottom: 0 },
            xAxis: { type: "category", show: false, data: series[0]?.values.map((_, i) => i) || [] },
            yAxis: { type: "value", show: false, min: "dataMin", max: "dataMax" },
            tooltip: { show: false },
            series: series.map((s) => ({
              type: "line",
              data: s.values.map((p) => p.v),
              showSymbol: false,
              smooth: 0.2,
              lineStyle: { width: 1.4, color: s.color },
              areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                  { offset: 0, color: hexAlpha(s.color, 0.28) },
                  { offset: 1, color: hexAlpha(s.color, 0) },
                ]),
              },
            })),
          }
        : {
            animationDuration: 400,
            animationDurationUpdate: 280,
            color: series.map((s) => s.color),
            grid: { left: 16, right: 18, top: 28, bottom: 52, containLabel: true },
            legend: {
              bottom: 0,
              left: 8,
              itemWidth: 10,
              itemHeight: 8,
              itemGap: 16,
              textStyle: { color: muted, fontSize: 12 },
              icon: "roundRect",
            },
            tooltip: {
              trigger: "axis",
              backgroundColor: surface,
              borderColor: line,
              borderWidth: 1,
              padding: [10, 12],
              textStyle: { color: ink, fontSize: 12 },
              axisPointer: {
                type: "cross",
                lineStyle: { color: accent, width: 1, opacity: 0.5 },
                crossStyle: { color: accent, opacity: 0.4 },
                label: { backgroundColor: surface2, color: ink, borderRadius: 4 },
              },
              formatter: (raw) => {
                const items = Array.isArray(raw) ? raw : [raw];
                if (!items.length) return "";
                const t = Number(items[0].value?.[0]);
                const rows = items
                  .map((it) => {
                    const v = Number(it.value?.[1]);
                    return `<div style="display:flex;justify-content:space-between;gap:20px;margin-top:4px">
                      <span><span style="display:inline-block;width:8px;height:8px;border-radius:99px;background:${it.color};margin-right:6px"></span>${it.seriesName}</span>
                      <b style="font-family:IBM Plex Mono,ui-monospace,monospace;font-weight:500">${fmt(v)}</b>
                    </div>`;
                  })
                  .join("");
                return `<div style="color:${muted};font-size:11px;margin-bottom:4px">${fmtClock(t)}</div>${rows}`;
              },
            },
            dataZoom: [
              { type: "inside", xAxisIndex: 0, filterMode: "none", zoomOnMouseWheel: true, moveOnMouseMove: true },
              {
                type: "slider",
                height: 16,
                bottom: 28,
                borderColor: "transparent",
                backgroundColor: accentDim,
                fillerColor: accentDim,
                handleSize: 12,
                handleStyle: { color: accent, borderColor: accent },
                moveHandleSize: 0,
                textStyle: { color: muted, fontSize: 10 },
                dataBackground: {
                  lineStyle: { color: accent, opacity: 0.4 },
                  areaStyle: { color: accentDim },
                },
                selectedDataBackground: {
                  lineStyle: { color: accent },
                  areaStyle: { color: accentDim },
                },
              },
            ],
            xAxis: {
              type: "time",
              boundaryGap: false,
              axisLine: { lineStyle: { color: line } },
              axisTick: { show: false },
              axisLabel: {
                color: muted,
                fontSize: 11,
                hideOverlap: true,
                formatter: (value: number) => fmtAxis(value, spanMs),
              },
              splitLine: { show: false },
            },
            yAxis: {
              type: "value",
              min: 0,
              axisLine: { show: false },
              axisTick: { show: false },
              axisLabel: {
                color: muted,
                fontSize: 11,
                formatter: (value: number) => fmt(value),
              },
              splitLine: { lineStyle: { color: line } },
            },
            series: series.map((s) => ({
              name: s.name,
              type: "line",
              showSymbol: false,
              symbol: "circle",
              symbolSize: 8,
              smooth: 0.18,
              sampling: "lttb",
              emphasis: { focus: "series", itemStyle: { borderWidth: 2, borderColor: surface } },
              lineStyle: { width: 2, color: s.color },
              itemStyle: { color: s.color },
              areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                  { offset: 0, color: hexAlpha(s.color, 0.28) },
                  { offset: 1, color: hexAlpha(s.color, 0.02) },
                ]),
              },
              data: s.values.map((p) => [p.t * 1000, p.v]),
            })),
          };
    chart.setOption(option, true);
  }, [series, empty, variant, points, theme]);

  if (empty) {
    return variant === "spark" ? <div className="spark" /> : <div className="empty">还没有足够的采样，等采集跑一会儿。</div>;
  }

  return (
    <div className={variant === "spark" ? "spark-chart" : "chart-wrap"} style={variant === "full" ? { height } : undefined}>
      <div ref={elRef} className="chart-el" />
    </div>
  );
}

export function Sparkline({ values, color = "#3ee0b2" }: { values: number[]; color?: string }) {
  return (
    <Chart
      variant="spark"
      height={42}
      series={[
        {
          name: "",
          color,
          values: values.map((v, i) => ({ t: i, v })),
        },
      ]}
    />
  );
}
