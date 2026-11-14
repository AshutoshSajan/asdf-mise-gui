import {
  AlertTriangle,
  ArrowRight,
  Boxes,
  ExternalLink,
  FolderGit2,
  Layers,
  Package,
  RefreshCw,
  Terminal,
  TriangleAlert,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { PageHeader, SectionTitle } from "../components/page-header";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/empty-state";
import { OperationStatusBadge, OperationStatusIcon } from "../components/operation-status";
import { useOperations, usePlugins, useProject, useStatus, useTools } from "../lib/query";
import { navigate } from "../lib/route";
import { basename, formatRelative, pluralize, shortenPath } from "../lib/format";
import { cn } from "../lib/utils";
import { ManagerService, type CurrentTool } from "../lib/bindings";
import type { ReactNode } from "react";

export function Dashboard() {
  const queryClient = useQueryClient();
  const status = useStatus(true);
  const tools = useTools();
  const plugins = usePlugins();
  const operations = useOperations();
  const defaultDir = status.data?.settings.defaultProjectDir ?? "";
  const project = useProject(defaultDir, false, status.isSuccess);

  const detection = status.data?.detection;
  const currentMap = new Map((project.data?.current ?? []).map((item) => [item.name, item]));
  const missing = project.data?.missing ?? [];
  const recentOperations = (operations.data ?? []).slice(0, 4);

  const refreshAll = () => {
    void queryClient.invalidateQueries();
  };

  const openDocs = () => {
    void ManagerService.OpenURL("https://asdf-vm.com/guide/getting-started.html");
  };

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Overview"
        title="Dashboard"
        description="One place to see your asdf engine, installed toolchains, and the project you are working on."
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => navigate("/project")}>
              <FolderGit2 className="size-3.5" />
              Open project
            </Button>
            <Button variant="ghost" size="icon" onClick={refreshAll} aria-label="Refresh dashboard">
              <RefreshCw className={cn("size-4", status.isFetching && "animate-spin")} />
            </Button>
          </>
        }
      />

      {!detection?.found ? (
        <Card className="overflow-hidden border-warn/25 bg-gradient-to-br from-warn/10 via-panel to-panel">
          <div className="flex flex-col gap-5 p-6 sm:flex-row sm:items-start">
            <div className="grid size-11 shrink-0 place-items-center rounded-xl border border-warn/25 bg-warn/10">
              <Terminal className="size-5 text-warn" />
            </div>
            <div className="min-w-0 flex-1">
              <h2 className="text-[15px] font-semibold text-ink">Connect asdf to get started</h2>
              <p className="mt-1.5 max-w-2xl text-[13px] leading-relaxed text-muted">
                asdf GUI drives the real asdf CLI. Install asdf and its Bash + Git dependencies, or point the app at an
                existing binary in Settings.
              </p>
              {detection?.error ? (
                <p className="mt-2 rounded-lg border border-danger/20 bg-danger/5 px-3 py-2 font-mono text-[11px] text-danger">
                  {detection.error}
                </p>
              ) : null}
              <div className="mt-4 flex flex-wrap items-center gap-2">
                <Button size="sm" onClick={() => navigate("/settings")}>
                  Open settings
                  <ArrowRight className="size-3.5" />
                </Button>
                <Button size="sm" variant="ghost" onClick={openDocs}>
                  asdf install guide
                  <ExternalLink className="size-3.5" />
                </Button>
              </div>
            </div>
          </div>
        </Card>
      ) : null}

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          label="asdf engine"
          value={detection?.version || "—"}
          hint={detection?.source ? `found via ${detection.source}` : "not detected"}
          icon={<Terminal className="size-4" />}
          tone={detection?.found ? "accent" : "danger"}
        />
        <StatCard
          label="Plugins"
          value={status.data ? String(status.data.pluginCount) : "—"}
          hint={`${status.data?.pluginCount ?? 0} ${pluralize(status.data?.pluginCount ?? 0, "plugin")} installed`}
          icon={<Package className="size-4" />}
        />
        <StatCard
          label="Installed versions"
          value={status.data ? String(status.data.versionCount) : "—"}
          hint={`across ${status.data?.toolCount ?? 0} ${pluralize(status.data?.toolCount ?? 0, "tool")}`}
          icon={<Layers className="size-4" />}
        />
        <StatCard
          label="Missing in project"
          value={project.isLoading ? "—" : String(missing.length)}
          hint={missing.length > 0 ? missing.slice(0, 3).join(", ") : "everything is installed"}
          icon={<TriangleAlert className="size-4" />}
          tone={missing.length > 0 ? "warn" : undefined}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.65fr)_minmax(300px,1fr)]">
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <div className="min-w-0">
                <CardTitle>Active project</CardTitle>
                <div className="mt-1 flex items-center gap-2">
                  <span className="truncate font-mono text-[11px] text-muted" title={project.data?.dir}>
                    {project.data?.dir ? shortenPath(project.data.dir, 62) : "resolving…"}
                  </span>
                  {project.data?.usingParent ? <Badge variant="warning">parent file</Badge> : null}
                </div>
              </div>
              <div className="flex items-center gap-1.5">
                {missing.length > 0 ? (
                  <Button
                    size="sm"
                    variant="primary"
                    onClick={() => navigate("/project")}
                    className="h-7 px-2.5 text-[11px]"
                  >
                    Install missing
                  </Button>
                ) : null}
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => navigate("/project")}
                  aria-label="Open project"
                >
                  <ArrowRight className="size-3.5" />
                </Button>
              </div>
            </CardHeader>
            <CardContent className="pt-3">
              {project.isLoading ? (
                <div className="space-y-2">
                  <div className="skeleton h-9 rounded-lg" />
                  <div className="skeleton h-9 rounded-lg" />
                </div>
              ) : project.data && (project.data.entries?.length ?? 0) > 0 ? (
                <div className="overflow-hidden rounded-lg border border-line-soft">
                  {(project.data.entries ?? []).slice(0, 7).map((entry) => {
                    const current = currentMap.get(entry.tool);
                    return (
                      <div
                        key={entry.tool}
                        className="flex items-center gap-3 border-b border-line-soft px-3 py-2.5 last:border-b-0"
                      >
                        <span className="w-32 shrink-0 truncate font-mono text-[12px] text-ink">{entry.tool}</span>
                        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted">
                          {(entry.versions ?? []).join(" ")}
                        </span>
                        {current ? <CurrentStatus current={current} /> : <Badge variant="outline">no resolution</Badge>}
                      </div>
                    );
                  })}
                  {(project.data.entries?.length ?? 0) > 7 ? (
                    <div className="px-3 py-2 text-[11px] text-faint">
                      +{(project.data.entries?.length ?? 0) - 7} more in {basename(project.data.filePath)}
                    </div>
                  ) : null}
                </div>
              ) : (
                <EmptyState
                  icon={<Boxes className="size-5" />}
                  title={project.data?.fileExists ? "No tools pinned" : "No .tool-versions yet"}
                  description={
                    project.data?.fileExists
                      ? "This project's file exists but does not pin any tools."
                      : "Open the project screen to create a .tool-versions file and pin your first toolchain."
                  }
                  action={
                    <Button size="sm" variant="secondary" onClick={() => navigate("/project")}>
                      Open project
                    </Button>
                  }
                  className="py-8"
                />
              )}

              {(project.data?.warnings ?? []).length > 0 ? (
                <div className="mt-3 space-y-1.5">
                  {(project.data?.warnings ?? []).map((warning) => (
                    <div key={warning} className="flex items-start gap-2 text-[11px] leading-relaxed text-warn">
                      <AlertTriangle className="mt-0.5 size-3 shrink-0" />
                      <span>{warning}</span>
                    </div>
                  ))}
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Recent operations</CardTitle>
              <Button variant="ghost" size="sm" className="h-7 text-[11px]" onClick={() => navigate("/operations")}>
                View all
                <ArrowRight className="size-3" />
              </Button>
            </CardHeader>
            <CardContent className="pt-2">
              {recentOperations.length === 0 ? (
                <EmptyState
                  icon={<Terminal className="size-5" />}
                  title="No operations yet"
                  description="Installs, uninstalls, and plugin changes will appear here with streamed logs."
                  className="py-7"
                />
              ) : (
                <div className="divide-y divide-line-soft">
                  {recentOperations.map((operation) => (
                    <button
                      key={operation.id}
                      onClick={() => navigate("/operations", { op: operation.id })}
                      className="flex w-full items-center gap-3 py-2.5 text-left transition hover:bg-elevated/40"
                    >
                      <OperationStatusIcon status={operation.status} />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-[12px] text-ink">{operation.title}</div>
                        <div className="mt-0.5 truncate font-mono text-[10px] text-faint">{operation.command}</div>
                      </div>
                      <div className="flex shrink-0 flex-col items-end gap-1">
                        <OperationStatusBadge status={operation.status} />
                        <span className="text-[10px] text-faint">{formatRelative(operation.createdAt)}</span>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Environment</CardTitle>
              <Badge variant={detection?.found ? "success" : "danger"}>
                {status.data?.environment ?? "detecting"}
              </Badge>
            </CardHeader>
            <CardContent className="space-y-3 pt-3">
              <InfoRow label="Platform" value={`${status.data?.os ?? "—"} / ${status.data?.arch ?? "—"}`} />
              <InfoRow label="Binary" value={detection?.binaryPath ?? "—"} mono />
              <InfoRow label="Data dir" value={detection?.dataDir ?? "—"} mono />
              <InfoRow label="Source" value={detection?.source ?? "—"} />
              <InfoRow label="Default project" value={status.data?.settings.defaultProjectDir || "not set"} mono />
              <div className="pt-1">
                <Button variant="secondary" size="sm" className="w-full" onClick={() => navigate("/settings")}>
                  Configure engine
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Quick actions</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-2 pt-3">
              <QuickAction icon={<Package className="size-4" />} label="Manage plugins" onClick={() => navigate("/plugins")} />
              <QuickAction icon={<Boxes className="size-4" />} label="Browse installed tools" onClick={() => navigate("/tools")} />
              <QuickAction icon={<Terminal className="size-4" />} label="Open operation logs" onClick={() => navigate("/operations")} />
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  hint,
  icon,
  tone,
}: {
  label: string;
  value: string;
  hint: string;
  icon: ReactNode;
  tone?: "accent" | "warn" | "danger";
}) {
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-semibold uppercase tracking-[0.13em] text-faint">{label}</span>
        <span
          className={cn(
            "grid size-7 place-items-center rounded-lg border border-line bg-elevated text-muted",
            tone === "accent" && "border-accent/25 bg-accent/10 text-accent",
            tone === "warn" && "border-warn/25 bg-warn/10 text-warn",
            tone === "danger" && "border-danger/25 bg-danger/10 text-danger",
          )}
        >
          {icon}
        </span>
      </div>
      <div className="mt-2.5 font-mono text-[21px] font-medium leading-none tracking-tight text-ink">{value}</div>
      <div className="mt-2 truncate text-[11px] text-muted" title={hint}>
        {hint}
      </div>
    </Card>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="shrink-0 text-[11px] text-faint">{label}</span>
      <span
        className={cn("min-w-0 truncate text-right text-[11px] text-muted", mono && "font-mono")}
        title={value}
      >
        {value}
      </span>
    </div>
  );
}

function QuickAction({
  icon,
  label,
  onClick,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="flex items-center gap-2.5 rounded-lg border border-line-soft bg-canvas/40 px-3 py-2.5 text-left text-[12px] text-muted transition hover:border-line hover:bg-elevated hover:text-ink"
    >
      <span className="text-faint">{icon}</span>
      <span className="flex-1">{label}</span>
      <ArrowRight className="size-3.5 text-faint" />
    </button>
  );
}

function CurrentStatus({ current }: { current: CurrentTool }) {
  if (current.system) return <Badge variant="info">system</Badge>;
  if (current.installed) return <Badge variant="success">active</Badge>;
  return <Badge variant="danger">missing</Badge>;
}
