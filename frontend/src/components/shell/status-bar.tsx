import { Cpu, Database, FolderOpen } from "lucide-react";
import { useOperations, useStatus } from "../../lib/query";
import { navigate } from "../../lib/route";
import { shortenPath } from "../../lib/format";

export function StatusBar() {
  const status = useStatus();
  const operations = useOperations();
  const active = (operations.data ?? []).filter(
    (operation) =>
      operation.status === "queued" || operation.status === "running" || operation.status === "canceling",
  ).length;

  return (
    <footer className="flex h-7 shrink-0 items-center justify-between gap-4 border-t border-line bg-surface/70 px-4 font-mono text-[10px] text-faint">
      <div className="flex min-w-0 items-center gap-4">
        <span className="flex items-center gap-1.5">
          <Cpu className="size-3" />
          {status.data?.environment ?? "detecting"}
        </span>
        <span className="flex min-w-0 items-center gap-1.5" title={status.data?.detection.dataDir}>
          <Database className="size-3" />
          <span className="truncate">{shortenPath(status.data?.detection.dataDir ?? "", 40)}</span>
        </span>
        <span className="hidden items-center gap-1.5 lg:flex">
          <FolderOpen className="size-3" />
          {status.data?.settings.defaultProjectDir ? shortenPath(status.data.settings.defaultProjectDir, 34) : "no default project"}
        </span>
      </div>
      <button
        onClick={() => navigate("/operations")}
        className="flex items-center gap-1.5 transition hover:text-muted"
      >
        <span className={active > 0 ? "text-info" : undefined}>
          {active > 0 ? `${active} operation${active === 1 ? "" : "s"} active` : "queue idle"}
        </span>
      </button>
    </footer>
  );
}
