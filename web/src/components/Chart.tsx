import { useEffect, useRef } from "react";
import * as echarts from "echarts/core";
import { LineChart } from "echarts/charts";
import { AxisPointerComponent, DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { EChartsCoreOption } from "echarts/core";

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
              textStyle: { color: "#8393a4", fontSize: 12 },
              icon: "roundRect",
            },
            tooltip: {
              trigger: "axis",
              backgroundColor: "rgba(12, 18, 24, 0.94)",
              borderColor: "rgba(148, 175, 196, 0.16)",
              borderWidth: 1,
              padding: [10, 12],
              textStyle: { color: "#e8eef4", fontSize: 12 },
              axisPointer: {
                type: "cross",
                lineStyle: { color: "rgba(62, 224, 178, 0.45)", width: 1 },
                crossStyle: { color: "rgba(62, 224, 178, 0.35)" },
                label: { backgroundColor: "#161f28", color: "#e8eef4", borderRadius: 4 },
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
                return `<div style="color:#8393a4;font-size:11px;margin-bottom:4px">${fmtClock(t)}</div>${rows}`;
              },
            },
            dataZoom: [
              { type: "inside", xAxisIndex: 0, filterMode: "none", zoomOnMouseWheel: true, moveOnMouseMove: true },
              {
                type: "slider",
                height: 16,
                bottom: 28,
                borderColor: "transparent",
                backgroundColor: "rgba(255,255,255,0.04)",
                fillerColor: "rgba(62, 224, 178, 0.14)",
                handleSize: 12,
                handleStyle: { color: "#3ee0b2", borderColor: "#3ee0b2" },
                moveHandleSize: 0,
                textStyle: { color: "#8393a4", fontSize: 10 },
                dataBackground: {
                  lineStyle: { color: "rgba(62, 224, 178, 0.35)" },
                  areaStyle: { color: "rgba(62, 224, 178, 0.08)" },
                },
                selectedDataBackground: {
                  lineStyle: { color: "#3ee0b2" },
                  areaStyle: { color: "rgba(62, 224, 178, 0.18)" },
                },
              },
            ],
            xAxis: {
              type: "time",
              boundaryGap: false,
              axisLine: { lineStyle: { color: "rgba(148, 175, 196, 0.18)" } },
              axisTick: { show: false },
              axisLabel: {
                color: "#7d8c9c",
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
                color: "#7d8c9c",
                fontSize: 11,
                formatter: (value: number) => fmt(value),
              },
              splitLine: { lineStyle: { color: "rgba(140, 170, 190, 0.1)" } },
            },
            series: series.map((s) => ({
              name: s.name,
              type: "line",
              showSymbol: false,
              symbol: "circle",
              symbolSize: 8,
              smooth: 0.18,
              sampling: "lttb",
              emphasis: { focus: "series", itemStyle: { borderWidth: 2, borderColor: "#0b1014" } },
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
  }, [series, empty, variant, points]);

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
