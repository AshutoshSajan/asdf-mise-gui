import { useEffect, useMemo, useState } from "react";
import {
  Ban,
  Clock3,
  ExternalLink,
  FolderOpen,
  History,
  ListFilter,
  Loader2,
  Play,
  Search,
  Terminal,
  Trash2,
  XCircle,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "../components/page-header";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { EmptyState, InlineEmpty } from "../components/empty-state";
import { Input } from "../components/ui/input";
import { useToast } from "../components/ui/toast";
import { CopyButton } from "../components/copy-button";
import { OperationOutput } from "../components/operation-output";
import {
  OperationStatusBadge,
  OperationStatusIcon,
  isActiveOperation,
} from "../components/operation-status";
import { useOperation, useOperations, queryKeys } from "../lib/query";
import {
  basename,
  errorMessage,
  formatDuration,
  formatRelative,
  pluralize,
  shortenPath,
} from "../lib/format";
import { cn } from "../lib/utils";
import { OperationService, type Operation } from "../lib/bindings";
import { navigate } from "../lib/route";

type Filter = "all" | "active" | "finished";

export function Operations({ selectedID }: { selectedID: string | null }) {
  const operations = useOperations();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [selected, setSelected] = useState(selectedID ?? "");

  useEffect(() => {
    if (selectedID) setSelected(selectedID);
  }, [selectedID]);

  const all = operations.data ?? [];
  const rows = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return all.filter((operation) => {
      if (filter === "active" && !isActiveOperation(operation)) return false;
      if (filter === "finished" && isActiveOperation(operation)) return false;
      if (!needle) return true;
      return (
        operation.title.toLowerCase().includes(needle) ||
        operation.command.toLowerCase().includes(needle) ||
        operation.kind.toLowerCase().includes(needle) ||
        operation.dir.toLowerCase().includes(needle)
      );
    });
  }, [all, filter, query]);

  const activeIDs = useMemo(() => new Set(all.filter(isActiveOperation).map((operation) => operation.id)), [all]);
  const effectiveID = selected && all.some((operation) => operation.id === selected)
    ? selected
    : all[0]?.id ?? null;

  const select = (id: string) => {
    setSelected(id);
    navigate("/operations", { op: id });
  };

  const clearFinished = async () => {
    try {
      const removed = await OperationService.ClearFinished();
      toast({
        title: removed > 0 ? `Cleared ${removed} ${pluralize(removed, "operation")}` : "Nothing to clear",
        description: removed > 0 ? "Finished operations were removed from this session." : "Only finished operations can be cleared.",
        variant: "info",
      });
      if (selected && !activeIDs.has(selected)) setSelected("");
      void queryClient.invalidateQueries({ queryKey: queryKeys.operations });
    } catch (error) {
      toast({ title: "Could not clear operations", description: errorMessage(error), variant: "error" });
    }
  };

  const openLogsDir = async () => {
    try {
      await OperationService.OpenLogsDir();
    } catch (error) {
      toast({ title: "Could not open logs folder", description: errorMessage(error), variant: "error" });
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Execution"
        title="Operations"
        description="Every asdf mutation is serialized through one queue. Streamed stdout and stderr stay attached to each run, with a full log on disk."
        actions={
          <>
            <Button variant="ghost" size="sm" onClick={openLogsDir}>
              <FolderOpen className="size-3.5" />
              Logs folder
            </Button>
            <Button variant="secondary" size="sm" onClick={clearFinished} disabled={all.length === 0}>
              <Trash2 className="size-3.5" />
              Clear finished
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
            placeholder="Search operations, commands, or directories…"
            className="pl-9"
            aria-label="Search operations"
          />
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-line bg-canvas/50 p-1">
          {(
            [
              ["all", "All"],
              ["active", "Active"],
              ["finished", "Finished"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              onClick={() => setFilter(value)}
              className={cn(
                "flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-[11px] font-medium transition",
                filter === value ? "bg-elevated text-ink" : "text-muted hover:text-ink",
              )}
            >
              {value === "active" ? <Loader2 className="size-3" /> : value === "finished" ? <History className="size-3" /> : <ListFilter className="size-3" />}
              {label}
              <span className="font-mono text-[10px] text-faint">
                {value === "all"
                  ? all.length
                  : value === "active"
                    ? all.filter(isActiveOperation).length
                    : all.filter((operation) => !isActiveOperation(operation)).length}
              </span>
            </button>
          ))}
        </div>
      </div>

      {operations.isError ? (
        <InlineEmpty>{errorMessage(operations.error)}</InlineEmpty>
      ) : operations.isLoading ? (
        <Card className="p-5">
          <div className="space-y-3">
            <div className="skeleton h-12 rounded-lg" />
            <div className="skeleton h-12 rounded-lg" />
            <div className="skeleton h-12 rounded-lg" />
          </div>
        </Card>
      ) : all.length === 0 ? (
        <EmptyState
          icon={<Terminal className="size-5" />}
          title="No operations yet"
          description="Install a version, add a plugin, or pin a toolchain. Each asdf invocation is recorded here with its live output."
          action={
            <Button size="sm" variant="secondary" onClick={() => navigate("/tools")}>
              Go to Tools
            </Button>
          }
        />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[minmax(300px,0.85fr)_minmax(0,1.6fr)]">
          <Card className="overflow-hidden">
            {rows.length === 0 ? (
              <EmptyState
                icon={<Search className="size-5" />}
                title="No operations match this view"
                description="Clear the search or switch back to All."
                className="py-8"
              />
            ) : (
              <div className="max-h-[calc(100vh-280px)] divide-y divide-line-soft overflow-auto">
                {rows.map((operation) => (
                  <OperationRow
                    key={operation.id}
                    operation={operation}
                    active={operation.id === effectiveID}
                    onClick={() => select(operation.id)}
                  />
                ))}
              </div>
            )}
          </Card>

          <OperationDetail id={effectiveID} />
        </div>
      )}
    </div>
  );
}

function OperationRow({
  operation,
  active,
  onClick,
}: {
  operation: Operation;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "flex w-full items-start gap-3 px-4 py-3 text-left transition",
        active ? "bg-elevated/70" : "hover:bg-elevated/35",
      )}
    >
      <div className="mt-0.5">
        <OperationStatusIcon status={operation.status} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-[12px] font-medium text-ink">{operation.title}</span>
          {active ? <span className="ml-auto h-1.5 w-1.5 shrink-0 rounded-full bg-accent" /> : null}
        </div>
        <div className="mt-0.5 truncate font-mono text-[10px] text-faint" title={operation.command}>
          {operation.command}
        </div>
        <div className="mt-1 flex items-center gap-2 text-[10px] text-faint">
          <span>{formatRelative(operation.createdAt)}</span>
          {operation.dir ? (
            <>
              <span>·</span>
              <span className="truncate" title={operation.dir}>{basename(operation.dir) || shortenPath(operation.dir, 24)}</span>
            </>
          ) : null}
        </div>
      </div>
      <OperationStatusBadge status={operation.status} />
    </button>
  );
}

function OperationDetail({ id }: { id: string | null }) {
  const detail = useOperation(id);
  const queryClient = useQueryClient();
  const { toast } = useToast();

  if (!id) {
    return (
      <Card className="grid min-h-64 place-items-center">
        <EmptyState
          icon={<Play className="size-5" />}
          title="Select an operation"
          description="Choose a run on the left to inspect its command, environment, and streamed output."
        />
      </Card>
    );
  }

  if (detail.isError) {
    return <InlineEmpty>{errorMessage(detail.error)}</InlineEmpty>;
  }

  if (detail.isLoading || !detail.data) {
    return (
      <Card className="space-y-3 p-5">
        <div className="skeleton h-5 w-52 rounded" />
        <div className="skeleton h-9 rounded-lg" />
        <div className="skeleton h-40 rounded-xl" />
      </Card>
    );
  }

  const operation = detail.data.operation;
  const lines = detail.data.output ?? [];
  const live = isActiveOperation(operation);

  const cancel = async () => {
    try {
      await OperationService.Cancel(operation.id);
      toast({ title: "Cancel requested", description: operation.title, variant: "warning" });
      void queryClient.invalidateQueries({ queryKey: queryKeys.operation(operation.id) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.operations });
    } catch (error) {
      toast({ title: "Could not cancel operation", description: errorMessage(error), variant: "error" });
    }
  };

  const openLog = async () => {
    try {
      await OperationService.OpenLog(operation.id);
    } catch (error) {
      toast({ title: "Could not open log", description: errorMessage(error), variant: "error" });
    }
  };

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-line-soft px-5 py-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-[14px] font-semibold tracking-tight text-ink">{operation.title}</h2>
            <OperationStatusBadge status={operation.status} />
            {live ? <span className="text-[10px] uppercase tracking-[0.12em] text-info">live</span> : null}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] text-faint">
            <span className="font-mono">{operation.kind}</span>
            <span>{formatRelative(operation.createdAt)}</span>
            <span>took {formatDuration(operation.startedAt, operation.finishedAt)}</span>
            {operation.status === "failed" ? (
              <span className="text-danger">exit {operation.exitCode}</span>
            ) : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {live ? (
            <Button size="sm" variant="danger" className="h-7" onClick={cancel}>
              <Ban className="size-3" />
              Cancel
            </Button>
          ) : (
            <Button size="sm" variant="ghost" className="h-7" onClick={openLog}>
              <ExternalLink className="size-3" />
              Open log
            </Button>
          )}
          <CopyButton value={operation.command || `asdf ${operation.kind}`} size="sm" variant="ghost" className="h-7" />
        </div>
      </div>

      <div className="grid gap-3 border-b border-line-soft px-5 py-3 text-[11px] sm:grid-cols-2">
        <DetailRow label="Command" value={operation.command} mono />
        <DetailRow label="Directory" value={operation.dir || "—"} mono />
        <DetailRow label="Started" value={formatAbsolute(operation.startedAt)} />
        <DetailRow label="Log file" value={operation.logPath || "—"} mono />
      </div>

      {operation.error ? (
        <div className="mx-5 mt-4 flex items-start gap-2 rounded-lg border border-danger/25 bg-danger/5 px-3 py-2.5">
          <XCircle className="mt-0.5 size-3.5 shrink-0 text-danger" />
          <p className="whitespace-pre-wrap break-words font-mono text-[11px] leading-relaxed text-danger">
            {operation.error}
          </p>
        </div>
      ) : null}

      <div className="p-5">
        <div className="mb-2 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.13em] text-faint">
            <Terminal className="size-3" />
            Output
          </div>
          <div className="flex items-center gap-2 text-[10px] text-faint">
            {live ? (
              <span className="flex items-center gap-1 text-info">
                <span className="size-1.5 animate-pulse rounded-full bg-info" />
                streaming
              </span>
            ) : null}
            <span>{lines.length} {pluralize(lines.length, "line")}</span>
          </div>
        </div>
        <OperationOutput lines={lines} live={live} />
        {operation.status === "queued" ? (
          <div className="mt-2 flex items-center gap-1.5 text-[10px] text-faint">
            <Clock3 className="size-3" />
            Waiting for the current operation to finish.
          </div>
        ) : null}
      </div>
    </Card>
  );
}

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex min-w-0 items-baseline gap-2">
      <span className="w-16 shrink-0 text-[10px] uppercase tracking-[0.08em] text-faint">{label}</span>
      <span className={cn("min-w-0 truncate text-muted", mono && "font-mono text-[10.5px]")} title={value}>
        {value}
      </span>
    </div>
  );
}

function formatAbsolute(iso?: string): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
