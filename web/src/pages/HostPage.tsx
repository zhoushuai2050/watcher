import { useSearchParams } from "react-router-dom";
import PageHeader from "../components/PageHeader";
import ProcessesPage from "./ProcessesPage";
import ResourcesPage from "./ResourcesPage";

export default function HostPage() {
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") === "process" ? "process" : "resource";

  function setTab(next: "resource" | "process") {
    const copy = new URLSearchParams(params);
    if (next === "process") copy.set("tab", "process");
    else copy.delete("tab");
    setParams(copy, { replace: true });
  }

  return (
    <div className="stack">
      <PageHeader
        kicker="负载"
        title={tab === "process" ? "进程" : "资源"}
        desc={tab === "process" ? "按服务和容器看占用，点名称查看近 1 小时曲线。" : "CPU、内存、磁盘和网络曲线，可切换时间窗口。"}
      >
        <div className="range-tabs">
          <button className={tab === "resource" ? "active" : ""} onClick={() => setTab("resource")}>
            资源
          </button>
          <button className={tab === "process" ? "active" : ""} onClick={() => setTab("process")}>
            进程
          </button>
        </div>
      </PageHeader>
      {tab === "process" ? <ProcessesPage /> : <ResourcesPage />}
    </div>
  );
}
