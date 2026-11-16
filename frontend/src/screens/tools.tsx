import { useEffect, useMemo, useState } from "react";
import {
  Boxes,
  CheckCircle2,
  ChevronDown,
  CircleDashed,
  Download,
  FolderGit2,
  MapPin,
  MoreHorizontal,
  Package,
  Pin,
  RefreshCw,
  Search,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import type { ToolView, VersionComparison } from "../lib/bindings";
import { ProjectService, ToolsService } from "../lib/bindings";
import { useAvailableVersions, useStatus, useTools, useVersionComparisons } from "../lib/query";
import { useQueueOperation } from "../lib/actions";
import { navigate } from "../lib/route";
import { errorMessage, pluralize, shortenPath, splitVersions } from "../lib/format";
import { cn } from "../lib/utils";
import { PageHeader } from "../components/page-header";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Card } from "../components/ui/card";
import { EmptyState, InlineEmpty } from "../components/empty-state";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { Input, Label } from "../components/ui/input";
import { useToast } from "../components/ui/toast";
import { Hint } from "../components/ui/tooltip";

type Filter = "all" | "installed" | "missing" | "unset" | "outdated";

type InstallTarget = {
  tool: string;
  version: string;
};

type PinTarget = {
  tool: string;
  version: string;
  scope: "global" | "local";
  versions: string[];
};

type UninstallTarget = {
  tool: string;
  version: string;
};

export function Tools() {
  const tools = useTools();
  const comparisons = useVersionComparisons();
  const status = useStatus();
  const queue = useQueueOperation();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [installTarget, setInstallTarget] = useState<InstallTarget | null>(null);
  const [pinTarget, setPinTarget] = useState<PinTarget | null>(null);
  const [uninstallTarget, setUninstallTarget] = useState<UninstallTarget | null>(null);

  const engine = status.data?.engine ?? "asdf";
  const isMise = engine === "mise";
  const engineName = isMise ? "mise" : "asdf";
  const configName = isMise ? "mise config" : ".tool-versions";

  const comparisonMap = useMemo(
    () => new Map<string, VersionComparison>((comparisons.data?.items ?? []).map((item) => [item.tool, item])),
    [comparisons.data],
  );
  const latestMap = useMemo(
    () =>
      new Map(
        (comparisons.data?.items ?? [])
          .filter((item) => item.latest)
          .map((item) => [
            item.tool,
            { version: item.latest, installed: item.latestInstalled, known: true },
          ]),
      ),
    [comparisons.data],
  );

  const rows = useMemo(() => {
    const all = tools.data ?? [];
    const needle = query.trim().toLowerCase();
    return all.filter((tool) => {
      const missing = (tool.current?.length ?? 0) > 0 && !tool.currentInstalled && !tool.system;
      const unset = (tool.current?.length ?? 0) === 0;
      if (filter === "installed" && (tool.installed?.length ?? 0) === 0) return false;
      if (filter === "missing" && !missing) return false;
      if (filter === "unset" && !unset) return false;
      if (filter === "outdated" && comparisonMap.get(tool.name)?.status !== "outdated") return false;
      if (!needle) return true;
      return (
        tool.name.toLowerCase().includes(needle) ||
        (tool.installed ?? []).some((version) => version.toLowerCase().includes(needle)) ||
        (tool.current ?? []).some((version) => version.toLowerCase().includes(needle))
      );
    });
  }, [tools.data, query, filter, comparisonMap]);

  const totalVersions = useMemo(
    () => (tools.data ?? []).reduce((count, tool) => count + (tool.installed?.length ?? 0), 0),
    [tools.data],
  );
  const missingCount = useMemo(
    () =>
      (tools.data ?? []).filter(
        (tool) => (tool.current?.length ?? 0) > 0 && !tool.currentInstalled && !tool.system,
      ).length,
    [tools.data],
  );
  const outdatedCount = comparisons.data?.outdatedCount ?? 0;

  const openInstall = (tool?: ToolView, version?: string) => {
    setInstallTarget({
      tool: tool?.name ?? "",
      version: version ?? "",
    });
  };

  const openPin = (tool: string, version: string, scope: "global" | "local", versions: string[]) => {
    setPinTarget({ tool, version, scope, versions });
  };

  const showWhere = async (tool: string, version: string) => {
    try {
      const path = await ToolsService.Where(tool, version);
      toast({
        title: `${tool} ${version}`,
        description: path || "This version is not installed.",
        variant: "info",
      });
    } catch (error) {
      toast({ title: "Could not resolve install path", description: errorMessage(error), variant: "error" });
    }
  };

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ["tools"] });
    void queryClient.invalidateQueries({ queryKey: ["version-comparisons"] });
  };

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Toolchains"
        title="Tools"
        description="Installed versions, resolved project versions, and the install queue — all in one place. Only one mutation runs at a time."
        actions={
          <>
            <Hint label="Reload tools and latest versions">
              <Button variant="ghost" size="icon" onClick={refresh} aria-label="Refresh tools">
                <RefreshCw className={cn("size-4", (tools.isFetching || comparisons.isFetching) && "animate-spin")} />
              </Button>
            </Hint>
            <Button size="sm" onClick={() => openInstall()}>
              <Download className="size-3.5" />
              Install version
            </Button>
          </>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-52 flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-faint" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search tools or versions…"
            className="pl-9"
            aria-label="Search tools"
          />
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-line bg-canvas/50 p-1">
          {(
            [
              ["all", "All"],
              ["installed", "Installed"],
              ["missing", "Missing"],
              ["outdated", "Outdated"],
              ["unset", "Unset"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              onClick={() => setFilter(value)}
              className={cn(
                "rounded-md px-2.5 py-1.5 text-[11px] font-medium transition",
                filter === value ? "bg-elevated text-ink" : "text-muted hover:text-ink",
              )}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="hidden items-center gap-4 border-l border-line pl-4 text-[11px] text-faint md:flex">
          <span>
            <span className="font-mono text-ink">{tools.data?.length ?? "—"}</span> tools
          </span>
          <span>
            <span className="font-mono text-ink">{totalVersions}</span> versions
          </span>
          {missingCount > 0 ? (
            <span className="text-warn">
              <span className="font-mono">{missingCount}</span> missing
            </span>
          ) : null}
          {outdatedCount > 0 ? (
            <span className="text-info">
              <span className="font-mono">{outdatedCount}</span> outdated
            </span>
          ) : null}
        </div>
      </div>

      {comparisons.isError ? (
        <div className="flex items-start gap-2 rounded-lg border border-warn/20 bg-warn/5 px-3 py-2 text-[11px] leading-relaxed text-warn">
          <TriangleAlert className="mt-0.5 size-3 shrink-0" />
          <span>{errorMessage(comparisons.error)}</span>
        </div>
      ) : comparisons.data?.warning ? (
        <div className="flex items-start gap-2 rounded-lg border border-warn/20 bg-warn/5 px-3 py-2 text-[11px] leading-relaxed text-warn">
          <TriangleAlert className="mt-0.5 size-3 shrink-0" />
          <span>{comparisons.data.warning}</span>
        </div>
      ) : null}

      {tools.isError ? (
        <InlineEmpty>{errorMessage(tools.error)}</InlineEmpty>
      ) : tools.isLoading ? (
        <Card className="divide-y divide-line-soft">
          {Array.from({ length: 5 }).map((_, index) => (
            <div key={index} className="flex items-center gap-4 p-5">
              <div className="skeleton size-9 rounded-lg" />
              <div className="flex-1 space-y-2">
                <div className="skeleton h-3.5 w-32 rounded" />
                <div className="skeleton h-3 w-64 rounded" />
              </div>
            </div>
          ))}
        </Card>
      ) : rows.length === 0 ? (
        <EmptyState
          icon={<Boxes className="size-5" />}
          title={query || filter !== "all" ? "No tools match this view" : "No tools installed yet"}
          description={
            query || filter !== "all"
              ? "Clear the search or switch filters to see the rest of your toolchains."
              : isMise
                ? "Install your first version. Tools appear here as soon as mise resolves them from its registry or backends."
                : "Add a plugin, then install your first version. Tools appear here as soon as asdf knows about them."
          }
          action={
            query || filter !== "all" ? undefined : (
              <div className="flex flex-wrap items-center justify-center gap-2">
                <Button size="sm" variant="secondary" onClick={() => setInstallTarget({ tool: "", version: "" })}>
                  <Download className="size-3.5" />
                  Install a version
                </Button>
                {isMise ? null : (
                  <Button size="sm" variant="ghost" onClick={() => navigate("/plugins")}>
                    <Package className="size-3.5" />
                    Manage plugins
                  </Button>
                )}
              </div>
            )
          }
        />
      ) : (
        <Card className="overflow-hidden">
          <div className="divide-y divide-line-soft">
            {rows.map((tool) => (
              <ToolRow
                key={tool.name}
                tool={tool}
                latest={latestMap.get(tool.name)}
                comparison={comparisonMap.get(tool.name)}
                configName={configName}
                onInstall={openInstall}
                onPin={openPin}
                onUninstall={setUninstallTarget}
                onWhere={showWhere}
              />
            ))}
          </div>
        </Card>
      )}

      <InstallDialog
        open={Boolean(installTarget)}
        onOpenChange={(open) => {
          if (!open) setInstallTarget(null);
        }}
        tools={tools.data ?? []}
        initial={installTarget ?? { tool: "", version: "" }}
        engineName={engineName}
        isMise={isMise}
      />

      <PinDialog
        target={pinTarget}
        onClose={() => setPinTarget(null)}
        defaultDir={status.data?.settings.defaultProjectDir ?? ""}
        isMise={isMise}
      />

      <ConfirmDialog
        open={Boolean(uninstallTarget)}
        onOpenChange={(open) => {
          if (!open) setUninstallTarget(null);
        }}
        title={`Uninstall ${uninstallTarget?.tool ?? ""} ${uninstallTarget?.version ?? ""}?`}
        description={`The version is removed from the ${engineName} data directory. Project pins are not changed.`}
        confirmLabel="Uninstall"
        destructive
        loading={queue.isPending}
        onConfirm={() => {
          if (!uninstallTarget) return;
          queue.mutate(
            {
              run: () => ToolsService.Uninstall(uninstallTarget.tool, uninstallTarget.version),
            },
            { onSuccess: () => setUninstallTarget(null) },
          );
        }}
      />
    </div>
  );
}

function ToolRow({
  tool,
  latest,
  comparison,
  configName,
  onInstall,
  onPin,
  onUninstall,
  onWhere,
}: {
  tool: ToolView;
  latest?: { version: string; installed: boolean; known: boolean };
  comparison?: VersionComparison;
  configName: string;
  onInstall: (tool?: ToolView, version?: string) => void;
  onPin: (tool: string, version: string, scope: "global" | "local", versions: string[]) => void;
  onUninstall: (target: UninstallTarget) => void;
  onWhere: (tool: string, version: string) => void;
}) {
  const installed = tool.installed ?? [];
  const current = tool.current ?? [];
  const missing = current.length > 0 && !tool.currentInstalled && !tool.system;
  const unset = current.length === 0;
  const versionsForPin = installed.length > 0 ? installed : current;

  return (
    <div className="group px-4 py-4 transition hover:bg-elevated/20 sm:px-5">
      <div className="flex flex-wrap items-start gap-3">
        <div
          className={cn(
            "mt-0.5 grid size-9 shrink-0 place-items-center rounded-lg border border-line bg-elevated text-muted",
            missing && "border-warn/25 bg-warn/10 text-warn",
            tool.currentInstalled && !tool.system && "border-accent/20 bg-accent/8 text-accent",
          )}
        >
          <Package className="size-4" />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-[13px] font-medium text-ink">{tool.name}</span>
            <CurrentBadge tool={tool} />
            {comparison?.status === "outdated" ? (
              <Badge variant="info" className="text-[10px]">
                behind latest {comparison.latest}
              </Badge>
            ) : comparison?.status === "current" && comparison.latest ? (
              <Badge variant="outline" className="text-[10px]">
                latest {comparison.latest}
              </Badge>
            ) : latest?.known && !latest.installed ? (
              <Badge variant="outline" className="text-[10px]">
                latest {latest.version}
              </Badge>
            ) : null}
            {comparison?.latestInstalled && comparison.status === "outdated" ? (
              <Badge variant="success" className="text-[10px]">
                latest installed
              </Badge>
            ) : null}
          </div>

          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-faint">
            <span>
              {installed.length} {pluralize(installed.length, "version")} installed
            </span>
            {current.length > 0 ? (
              <span className="truncate">
                current <span className="font-mono text-muted">{current.join(" ")}</span>
                {tool.source ? <span className="text-faint"> · {shortenPath(tool.source, 44)}</span> : null}
              </span>
            ) : (
              <span>not resolved in this context</span>
            )}
          </div>

          {comparison && (comparison.status === "outdated" || comparison.status === "missing") ? (
            <p
              className={cn(
                "mt-1 text-[11px] leading-relaxed",
                comparison.status === "missing" ? "text-danger" : "text-info",
              )}
            >
              {comparison.reason}
            </p>
          ) : null}

          {installed.length > 0 ? (
            <div className="mt-2.5 flex flex-wrap gap-1.5">
              {installed.map((version) => {
                const isCurrent = current.includes(version) && tool.currentInstalled;
                return (
                  <DropdownMenu key={version}>
                    <DropdownMenuTrigger asChild>
                      <button
                        className={cn(
                          "inline-flex items-center gap-1 rounded-md border px-2 py-1 font-mono text-[11px] transition",
                          isCurrent
                            ? "border-accent/30 bg-accent/10 text-accent"
                            : "border-line bg-canvas/40 text-muted hover:border-line-strong hover:text-ink",
                        )}
                      >
                        {version}
                        <ChevronDown className="size-2.5 opacity-60" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                      <DropdownMenuLabel>{tool.name} {version}</DropdownMenuLabel>
                      <DropdownMenuItem onClick={() => onPin(tool.name, version, "global", versionsForPin)}>
                        <Pin className="size-3.5" />
                        Pin globally
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => onPin(tool.name, version, "local", versionsForPin)}>
                        <FolderGit2 className="size-3.5" />
                        Pin to project…
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => onWhere(tool.name, version)}>
                        <MapPin className="size-3.5" />
                        Show install path
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        destructive
                        onClick={() => onUninstall({ tool: tool.name, version })}
                      >
                        <Trash2 className="size-3.5" />
                        Uninstall
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                );
              })}
            </div>
          ) : (
            <div className="mt-2 text-[11px] text-faint">
              No versions installed. Choose an available version to get started.
            </div>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-1.5">
          {missing ? (
            <Button
              size="sm"
              variant="secondary"
              className="h-7 border-warn/30 text-warn hover:bg-warn/10 hover:text-warn"
              onClick={() => onInstall(tool, current[0])}
            >
              <Download className="size-3" />
              Install missing
            </Button>
          ) : null}
          {comparison?.status === "outdated" ? (
            <Button
              size="sm"
              variant="secondary"
              className="h-7 border-info/30 text-info hover:bg-info/10 hover:text-info"
              onClick={() => onInstall(tool, comparison.latest)}
            >
              <Download className="size-3" />
              Update
            </Button>
          ) : null}
          <Button size="sm" variant="secondary" className="h-7" onClick={() => onInstall(tool)}>
            <Download className="size-3" />
            Install
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={`More actions for ${tool.name}`}>
                <MoreHorizontal className="size-3.5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuLabel>Tool actions</DropdownMenuLabel>
              <DropdownMenuItem onClick={() => onInstall(tool)}>
                <Download className="size-3.5" />
                Install a version
              </DropdownMenuItem>
              {versionsForPin.length > 0 ? (
                <DropdownMenuItem onClick={() => onPin(tool.name, versionsForPin[0], "global", versionsForPin)}>
                  <Pin className="size-3.5" />
                  Pin version
                </DropdownMenuItem>
              ) : null}
              {comparison?.status === "outdated" && comparison.latest ? (
                <DropdownMenuItem
                  onClick={() =>
                    onPin(tool.name, comparison.latest, "global", [...versionsForPin, comparison.latest])
                  }
                >
                  <Pin className="size-3.5" />
                  Pin latest globally
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => navigate("/plugins")}>
                <Package className="size-3.5" />
                Open plugins
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {unset && installed.length > 0 ? (
        <div className="mt-2 pl-12 text-[11px] text-faint">
          Installed but not selected by any <span className="font-mono">{configName}</span> file.
        </div>
      ) : null}
    </div>
  );
}

function CurrentBadge({ tool }: { tool: ToolView }) {
  if (tool.system) {
    return (
      <Badge variant="info" className="gap-1">
        <CheckCircle2 className="size-3" />
        system
      </Badge>
    );
  }
  if (tool.currentInstalled && (tool.current?.length ?? 0) > 0) {
    return (
      <Badge variant="success" className="gap-1">
        <CheckCircle2 className="size-3" />
        active
      </Badge>
    );
  }
  if ((tool.current?.length ?? 0) > 0) {
    return (
      <Badge variant="danger" className="gap-1">
        <TriangleAlert className="size-3" />
        missing
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="gap-1">
      <CircleDashed className="size-3" />
      unset
    </Badge>
  );
}

function InstallDialog({
  open,
  onOpenChange,
  tools,
  initial,
  engineName,
  isMise,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tools: ToolView[];
  initial: InstallTarget;
  engineName: string;
  isMise: boolean;
}) {
  const queue = useQueueOperation();
  const [tool, setTool] = useState(initial.tool);
  const [version, setVersion] = useState(initial.version);
  const [versionQuery, setVersionQuery] = useState("");
  // Free-text tool names stay responsive while still debouncing registry
  // lookups, which may hit the network for every keystroke.
  const [availableFor, setAvailableFor] = useState(initial.tool.trim());
  const available = useAvailableVersions(availableFor || null);

  useEffect(() => {
    if (!open) return;
    setTool(initial.tool);
    setVersion(initial.version);
    setVersionQuery("");
    setAvailableFor(initial.tool.trim());
    // The dialog is intentionally reset on each open, not on every parent render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    const timer = window.setTimeout(() => setAvailableFor(tool.trim()), 400);
    return () => window.clearTimeout(timer);
  }, [tool]);

  const versions = useMemo(() => {
    const all = [...(available.data ?? [])].reverse();
    const needle = versionQuery.trim().toLowerCase();
    return needle ? all.filter((item) => item.toLowerCase().includes(needle)) : all;
  }, [available.data, versionQuery]);

  const canSubmit = Boolean(tool) && Boolean(version.trim()) && !queue.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Install a tool version</DialogTitle>
          <DialogDescription>
            Choose a tool and a version. The install is queued behind any other running {engineName} mutation.
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div className="grid gap-4 sm:grid-cols-[minmax(0,180px)_minmax(0,1fr)]">
            <div>
              <Label>Tool</Label>
              <Input
                value={tool}
                onChange={(event) => {
                  setTool(event.target.value);
                  setVersion("");
                  setVersionQuery("");
                }}
                placeholder={isMise ? "e.g. node, python, terraform" : "Choose or type a tool"}
                list="install-tool-names"
                className="font-mono"
                autoFocus
              />
              <datalist id="install-tool-names">
                {tools.map((item) => (
                  <option key={item.name} value={item.name} />
                ))}
              </datalist>
              <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                {isMise
                  ? "Type any tool in mise's registry or backends; an installed plugin is not required. Custom plugins are managed on the "
                  : "Type a tool name, or pick one you already use. Plugins are managed on the "}
                <button className="text-muted underline underline-offset-2 hover:text-ink" onClick={() => onOpenChange(false)}>Plugins</button>{" "}
                screen.
              </p>
            </div>
            <div>
              <Label>Version</Label>
              <Input
                value={version}
                onChange={(event) => setVersion(event.target.value)}
                placeholder={tool ? "e.g. 22.0.0, ref:abc123, latest" : "Choose a tool first"}
                disabled={!tool}
              />
              <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                You can type an exact version or pick one from the list below.
              </p>
            </div>
          </div>

          {tool ? (
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-2">
                <Label className="mb-0">Available versions</Label>
                {available.data ? (
                  <span className="text-[10px] text-faint">
                    {versions.length === available.data.length
                      ? `${available.data.length} versions`
                      : `${versions.length} of ${available.data.length}`}
                  </span>
                ) : null}
              </div>
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-faint" />
                <Input
                  value={versionQuery}
                  onChange={(event) => setVersionQuery(event.target.value)}
                  placeholder="Filter versions…"
                  className="pl-9"
                />
              </div>
              <div className="max-h-52 overflow-auto rounded-lg border border-line bg-canvas/40 p-1">
                {available.isLoading ? (
                  <div className="space-y-1.5 p-2">
                    <div className="skeleton h-6 rounded" />
                    <div className="skeleton h-6 rounded" />
                    <div className="skeleton h-6 rounded" />
                  </div>
                ) : available.isError ? (
                  <p className="px-3 py-4 text-xs leading-relaxed text-danger">{errorMessage(available.error)}</p>
                ) : versions.length === 0 ? (
                  <p className="px-3 py-4 text-xs text-faint">
                    {versionQuery ? "No versions match this filter." : `No versions were reported by ${isMise ? "mise" : "the plugin"}.`}
                  </p>
                ) : (
                  versions.slice(0, 200).map((item) => (
                    <button
                      key={item}
                      onClick={() => setVersion(item)}
                      className={cn(
                        "flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-left font-mono text-[12px] transition",
                        version === item ? "bg-accent/10 text-accent" : "text-muted hover:bg-elevated hover:text-ink",
                      )}
                    >
                      <span>{item}</span>
                      {version === item ? <CheckCircle2 className="size-3.5" /> : null}
                    </button>
                  ))
                )}
              </div>
            </div>
          ) : null}

          {available.isError ? (
            <p className="text-[11px] leading-relaxed text-faint">
              The version list could not be loaded. You can still type a version manually.
            </p>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={queue.isPending}>
            Cancel
          </Button>
          <Button
            disabled={!canSubmit}
            loading={queue.isPending}
            onClick={() => {
              if (!tool || !version.trim()) return;
              queue.mutate(
                { run: () => ToolsService.Install(tool, version.trim()) },
                { onSuccess: () => onOpenChange(false) },
              );
            }}
          >
            <Download className="size-3.5" />
            Queue install
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function PinDialog({
  target,
  onClose,
  defaultDir,
  isMise,
}: {
  target: PinTarget | null;
  onClose: () => void;
  defaultDir: string;
  isMise: boolean;
}) {
  const status = useStatus();
  const queue = useQueueOperation();
  const { toast } = useToast();
  const [version, setVersion] = useState("");
  const [scope, setScope] = useState<"global" | "local">("global");
  const [dir, setDir] = useState(defaultDir);
  const globalConfigPath = isMise
    ? status.data?.configFile
      ? shortenPath(status.data.configFile, 40)
      : "mise global config"
    : "~/.tool-versions";

  useEffect(() => {
    if (!target) return;
    setVersion(target.version);
    setScope(target.scope);
    setDir(defaultDir);
    // Only reset when a new pin target is opened.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target]);

  const chooseDir = async () => {
    try {
      const path = await ProjectService.ChooseDirectory();
      if (path) setDir(path);
    } catch (error) {
      toast({ title: "Could not choose directory", description: errorMessage(error), variant: "error" });
    }
  };

  const versions = splitVersions(version);
  const canSubmit = Boolean(target) && versions.length > 0 && (scope === "global" || Boolean(dir.trim()));

  return (
    <Dialog open={Boolean(target)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            Pin <span className="font-mono">{target?.tool}</span>
          </DialogTitle>
          <DialogDescription>
            {isMise ? (
              <>
                Global pins live in <span className="font-mono">{globalConfigPath}</span>. Local pins run{" "}
                <span className="font-mono">mise use</span> against the selected project, which keeps TOML formatting
                and install state authoritative.
              </>
            ) : (
              <>
                Global pins live in <span className="font-mono">~/.tool-versions</span>. Local pins write to the selected
                project's <span className="font-mono">.tool-versions</span>.
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div>
            <Label>Version</Label>
            <Input
              value={version}
              onChange={(event) => setVersion(event.target.value)}
              placeholder="e.g. 22.0.0"
              className="font-mono"
            />
            <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
              Space-separated versions create a fallback list. Installed versions:{" "}
              <span className="font-mono text-muted">{target?.versions.join(" ") || "none"}</span>
            </p>
          </div>

          <div>
            <Label>Scope</Label>
            <div className="grid grid-cols-2 gap-2">
              <button
                onClick={() => setScope("global")}
                className={cn(
                  "rounded-lg border px-3 py-2.5 text-left transition",
                  scope === "global"
                    ? "border-accent/35 bg-accent/10"
                    : "border-line bg-canvas/40 hover:border-line-strong",
                )}
              >
                <span className={cn("block text-[12px] font-medium", scope === "global" ? "text-accent" : "text-ink")}>
                  Global
                </span>
                <span className="mt-0.5 block truncate font-mono text-[10px] text-faint" title={status.data?.configFile}>
                  {globalConfigPath}
                </span>
              </button>
              <button
                onClick={() => setScope("local")}
                className={cn(
                  "rounded-lg border px-3 py-2.5 text-left transition",
                  scope === "local"
                    ? "border-accent/35 bg-accent/10"
                    : "border-line bg-canvas/40 hover:border-line-strong",
                )}
              >
                <span className={cn("block text-[12px] font-medium", scope === "local" ? "text-accent" : "text-ink")}>
                  Project
                </span>
                <span className="mt-0.5 block font-mono text-[10px] text-faint">
                  {isMise ? "mise.toml" : ".tool-versions"}
                </span>
              </button>
            </div>
          </div>

          {scope === "local" ? (
            <div>
              <Label>Project directory</Label>
              <div className="flex gap-2">
                <Input
                  value={dir}
                  onChange={(event) => setDir(event.target.value)}
                  placeholder="/path/to/project"
                  className="font-mono text-[12px]"
                />
                <Button variant="secondary" onClick={chooseDir}>
                  <FolderGit2 className="size-3.5" />
                  Choose
                </Button>
              </div>
            </div>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={queue.isPending}>
            Cancel
          </Button>
          <Button
            disabled={!canSubmit}
            loading={queue.isPending}
            onClick={() => {
              if (!target) return;
              queue.mutate(
                { run: () => ToolsService.SetVersion(target.tool, versions, scope, scope === "local" ? dir : "") },
                { onSuccess: onClose },
              );
            }}
          >
            <Pin className="size-3.5" />
            Pin {scope === "global" ? "globally" : "to project"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
