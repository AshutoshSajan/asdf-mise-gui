import {
  Boxes,
  FolderGit2,
  LayoutDashboard,
  Package,
  Settings2,
  Terminal,
} from "lucide-react";
import { cn } from "../../lib/utils";
import { navigate, useRoute } from "../../lib/route";
import { useStatus, useOperations } from "../../lib/query";
import { LogoMark } from "../logo";
import { Badge } from "../ui/badge";

const NAV_ITEMS = [
  { path: "/", label: "Dashboard", icon: LayoutDashboard },
  { path: "/tools", label: "Tools", icon: Boxes },
  { path: "/plugins", label: "Plugins", icon: Package },
  { path: "/project", label: "Project", icon: FolderGit2 },
  { path: "/operations", label: "Operations", icon: Terminal },
  { path: "/settings", label: "Settings", icon: Settings2 },
];

export function Sidebar() {
  const route = useRoute();
  const status = useStatus();
  const operations = useOperations();
  const activeOperations = (operations.data ?? []).filter(
    (operation) =>
      operation.status === "queued" || operation.status === "running" || operation.status === "canceling",
  ).length;

  return (
    <aside className="flex w-[228px] shrink-0 flex-col border-r border-line bg-surface/60">
      <div className="drag-region flex h-[64px] items-center gap-3 px-5">
        <LogoMark className="size-8 shrink-0" />
        <div className="leading-none">
          <div className="flex items-baseline gap-1.5">
            <span className="text-[15px] font-semibold tracking-[-0.02em] text-ink">asdf</span>
            <span className="rounded bg-elevated px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.12em] text-muted">
              GUI
            </span>
          </div>
          <div className="mt-1 text-[10px] uppercase tracking-[0.14em] text-faint">version manager</div>
        </div>
      </div>

      <nav className="flex-1 space-y-0.5 px-3 pt-3">
        {NAV_ITEMS.map((item) => {
          const active = route.path === item.path;
          const Icon = item.icon;
          return (
            <button
              key={item.path}
              onClick={() => navigate(item.path)}
              className={cn(
                "group relative flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-[13px] font-medium transition",
                active
                  ? "bg-elevated text-ink shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
                  : "text-muted hover:bg-elevated/60 hover:text-ink",
              )}
            >
              {active ? (
                <span className="absolute left-0 top-1/2 h-4 w-[2px] -translate-y-1/2 rounded-full bg-accent" />
              ) : null}
              <Icon className={cn("size-4 shrink-0", active ? "text-accent" : "text-faint group-hover:text-muted")} />
              <span className="flex-1">{item.label}</span>
              {item.path === "/operations" && activeOperations > 0 ? (
                <span className="grid size-5 place-items-center rounded-full bg-info/15 text-[10px] font-semibold text-info">
                  {activeOperations}
                </span>
              ) : null}
            </button>
          );
        })}
      </nav>

      <div className="p-3">
        <button
          onClick={() => navigate("/settings")}
          className="w-full rounded-xl border border-line bg-canvas/45 p-3 text-left transition hover:border-line-strong"
        >
          <div className="flex items-center justify-between gap-2">
            <span className="text-[10px] font-semibold uppercase tracking-[0.13em] text-faint">Engine</span>
            <span
              className={cn(
                "size-1.5 rounded-full",
                status.data?.detection.found ? "bg-accent shadow-[0_0_8px_var(--accent)]" : "bg-danger",
              )}
            />
          </div>
          <div className="mt-2 flex items-center gap-2">
            <span className="font-mono text-[12px] text-ink">
              {status.data?.detection.found ? status.data.detection.version || "asdf" : "asdf missing"}
            </span>
            {status.data?.detection.source ? (
              <Badge variant="outline" className="px-1.5 py-0 text-[9px] uppercase tracking-wide">
                {status.data.detection.source}
              </Badge>
            ) : null}
          </div>
          <div className="mt-1 truncate font-mono text-[10px] text-faint" title={status.data?.detection.dataDir}>
            {status.data?.detection.dataDir || "no data dir"}
          </div>
        </button>
      </div>
    </aside>
  );
}
