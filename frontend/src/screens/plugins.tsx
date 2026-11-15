import { useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  GitBranch,
  Package,
  Plus,
  RefreshCw,
  Search,
  Store,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
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
import { Input, Label } from "../components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { Hint } from "../components/ui/tooltip";
import { useToast } from "../components/ui/toast";
import {
  usePluginCatalog,
  usePlugins,
  useStatus,
  queryKeys,
} from "../lib/query";
import { useQueueOperation } from "../lib/actions";
import { errorMessage, hostnameFromURL, pluralize, shortenPath } from "../lib/format";
import { cn } from "../lib/utils";
import { ManagerService, PluginsService, type AvailablePlugin, type Plugin } from "../lib/bindings";
import { navigate } from "../lib/route";

type RemoveTarget = { name: string };

const PAGE_SIZE = 50;

export function Plugins() {
  const status = useStatus();
  const plugins = usePlugins();
  const queryClient = useQueryClient();
  const queue = useQueueOperation();
  const { toast } = useToast();

  const [query, setQuery] = useState("");
  const [tab, setTab] = useState("installed");
  const [addOpen, setAddOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<RemoveTarget | null>(null);

  const detection = status.data?.detection;
  const engine = status.data?.engine || detection?.engine || "asdf";
  const isMise = engine === "mise";

  const installed = plugins.data ?? [];
  const rows = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return installed;
    return installed.filter(
      (plugin) =>
        plugin.name.toLowerCase().includes(needle) ||
        plugin.url.toLowerCase().includes(needle) ||
        plugin.ref.toLowerCase().includes(needle),
    );
  }, [installed, query]);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.plugins });
  };

  const updateAll = () => {
    queue.mutate(
      { run: () => PluginsService.Update("") },
      { onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.plugins }) },
    );
  };

  const openDocs = () => {
    void ManagerService.OpenURL(
      isMise ? "https://mise.jdx.dev/registry.html" : "https://asdf-vm.com/manage-plugins.html",
    );
  };

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Registry"
        title="Plugins"
        description={`Plugins provide the version list and install recipes for each tool. Add from ${
          isMise ? "the mise registry" : "the official asdf registry"
        } or a Git URL.`}
        actions={
          <>
            <Hint label="Reload installed plugins">
              <Button variant="ghost" size="icon" onClick={refresh} aria-label="Refresh plugins">
                <RefreshCw className={cn("size-4", plugins.isFetching && "animate-spin")} />
              </Button>
            </Hint>
            <Button variant="secondary" size="sm" onClick={updateAll} disabled={queue.isPending || installed.length === 0}>
              <RefreshCw className={cn("size-3.5", queue.isPending && "animate-spin")} />
              Update all
            </Button>
            <Button size="sm" onClick={() => setAddOpen(true)}>
              <Plus className="size-3.5" />
              Add plugin
            </Button>
          </>
        }
      />

      {!detection?.found ? (
        <Card className="border-warn/25 bg-warn/5 p-4">
          <div className="flex items-start gap-3">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warn" />
            <div className="text-[12px] leading-relaxed text-muted">
              {engine} was not detected. Plugin commands will fail until the engine is configured on the{" "}
              <button className="text-ink underline underline-offset-2" onClick={() => navigate("/settings")}>
                Settings
              </button>{" "}
              screen.
            </div>
          </div>
        </Card>
      ) : null}

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-52 flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-faint" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search plugins by name, URL, or ref…"
            className="pl-9"
            aria-label="Search plugins"
          />
        </div>
        <div className="hidden items-center gap-4 border-l border-line pl-4 text-[11px] text-faint sm:flex">
          <span>
            <span className="font-mono text-ink">{installed.length}</span> {pluralize(installed.length, "plugin")}
          </span>
        </div>
      </div>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="installed">
            Installed
            <span className="ml-1.5 font-mono text-[10px] text-faint">{installed.length}</span>
          </TabsTrigger>
          <TabsTrigger value="catalog">
            Registry
          </TabsTrigger>
        </TabsList>

        <TabsContent value="installed" className="mt-4">
          {plugins.isError ? (
            <InlineEmpty>{errorMessage(plugins.error)}</InlineEmpty>
          ) : plugins.isLoading ? (
            <Card className="divide-y divide-line-soft">
              {Array.from({ length: 4 }).map((_, index) => (
                <div key={index} className="flex items-center gap-4 p-5">
                  <div className="skeleton size-9 rounded-lg" />
                  <div className="flex-1 space-y-2">
                    <div className="skeleton h-3.5 w-28 rounded" />
                    <div className="skeleton h-3 w-56 rounded" />
                  </div>
                </div>
              ))}
            </Card>
          ) : rows.length === 0 ? (
            <EmptyState
              icon={<Package className="size-5" />}
              title={query ? "No plugins match this search" : "No plugins installed yet"}
              description={
                query
                  ? "Try a different name, URL, or ref."
                  : `Add a plugin from ${isMise ? "the mise registry" : "the official asdf registry"}, then install a version from the Tools screen.`
              }
              action={
                <Button size="sm" onClick={() => (query ? setQuery("") : setAddOpen(true))}>
                  {query ? "Clear search" : <><Plus className="size-3.5" />Add your first plugin</>}
                </Button>
              }
            />
          ) : (
            <Card className="overflow-hidden">
              <div className="divide-y divide-line-soft">
                {rows.map((plugin) => (
                  <PluginRow
                    key={plugin.name}
                    plugin={plugin}
                    busy={queue.isPending}
                    onUpdate={() =>
                      queue.mutate(
                        { run: () => PluginsService.Update(plugin.name) },
                        { onSuccess: refresh },
                      )
                    }
                    onRemove={() => setRemoveTarget({ name: plugin.name })}
                    onManage={() => navigate("/tools")}
                  />
                ))}
              </div>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="catalog" className="mt-4">
          <PluginCatalog
            installedNames={installed.map((plugin) => plugin.name)}
            onAdded={refresh}
            onAddManual={() => setAddOpen(true)}
            isMise={isMise}
          />
        </TabsContent>
      </Tabs>

      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-line-soft bg-canvas/30 px-4 py-3">
        <p className="text-[11px] leading-relaxed text-faint">
          Plugins are cloned with Git. Add a fork or a specific repository when the registry does not have what you need.
        </p>
        <Button variant="ghost" size="sm" className="h-7 text-[11px]" onClick={openDocs}>
          Plugin documentation
          <ExternalLink className="size-3" />
        </Button>
      </div>

      <AddPluginDialog open={addOpen} onOpenChange={setAddOpen} onAdded={refresh} />

      <ConfirmDialog
        open={Boolean(removeTarget)}
        onOpenChange={(open) => {
          if (!open) setRemoveTarget(null);
        }}
        title={`Remove plugin ${removeTarget?.name ?? ""}?`}
        description={`The plugin is removed from the ${engine} data directory. Installed tool versions stay on disk, but they are no longer discoverable until a plugin with the same name is added again.`}
        confirmLabel="Remove plugin"
        destructive
        loading={queue.isPending}
        onConfirm={() => {
          if (!removeTarget) return;
          queue.mutate(
            { run: () => PluginsService.Remove(removeTarget.name) },
            { onSuccess: () => setRemoveTarget(null) },
          );
        }}
      />
    </div>
  );
}

function PluginRow({
  plugin,
  busy,
  onUpdate,
  onRemove,
  onManage,
}: {
  plugin: Plugin;
  busy: boolean;
  onUpdate: () => void;
  onRemove: () => void;
  onManage: () => void;
}) {
  return (
    <div className="group flex flex-wrap items-center gap-3 px-4 py-4 transition hover:bg-elevated/20 sm:px-5">
      <div className="grid size-9 shrink-0 place-items-center rounded-lg border border-line bg-elevated text-accent">
        <Package className="size-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-[13px] font-medium text-ink">{plugin.name}</span>
          {plugin.ref ? (
            <Badge variant="outline" className="gap-1 text-[10px]">
              <GitBranch className="size-2.5" />
              {plugin.ref}
            </Badge>
          ) : null}
        </div>
        <div className="mt-1 truncate font-mono text-[11px] text-faint" title={plugin.url}>
          {plugin.url ? shortenPath(plugin.url, 72) : "no remote URL reported"}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        <Button size="sm" variant="ghost" className="h-7 text-[11px]" onClick={onManage}>
          Versions
        </Button>
        <Button size="sm" variant="secondary" className="h-7" onClick={onUpdate} disabled={busy}>
          <RefreshCw className="size-3" />
          Update
        </Button>
        <Hint label={`Remove ${plugin.name}`}>
          <Button
            size="sm"
            variant="ghost"
            className="h-7 w-7 p-0 text-faint hover:text-danger"
            onClick={onRemove}
            aria-label={`Remove ${plugin.name}`}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </Hint>
      </div>
    </div>
  );
}

function PluginCatalog({
  installedNames,
  onAdded,
  onAddManual,
  isMise,
}: {
  installedNames: string[];
  onAdded: () => void;
  onAddManual: () => void;
  isMise: boolean;
}) {
  const catalog = usePluginCatalog(isMise ? "mise" : "asdf");
  const queue = useQueueOperation();
  const { toast } = useToast();
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [pending, setPending] = useState<string | null>(null);

  const installed = useMemo(() => new Set(installedNames), [installedNames]);
  const rows = useMemo(() => {
    const all = catalog.data ?? [];
    const needle = query.trim().toLowerCase();
    if (!needle) return all;
    return all.filter(
      (entry) => entry.name.toLowerCase().includes(needle) || entry.url.toLowerCase().includes(needle),
    );
  }, [catalog.data, query]);

  useEffect(() => {
    setPage(1);
  }, [query]);

  const pageCount = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
  const currentPage = Math.min(page, pageCount);
  const pageStart = (currentPage - 1) * PAGE_SIZE;
  const pageRows = rows.slice(pageStart, pageStart + PAGE_SIZE);

  useEffect(() => {
    if (catalog.isError) {
      toast({
        title: "Registry unavailable",
        description: errorMessage(catalog.error),
        variant: "warning",
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [catalog.isError]);

  const add = (entry: AvailablePlugin) => {
    setPending(entry.name);
    queue.mutate(
      { run: () => PluginsService.Add(entry.name, entry.url) },
      {
        onSuccess: () => {
          setPending(null);
          onAdded();
        },
        onError: () => setPending(null),
      },
    );
  };

  if (catalog.isLoading) {
    return (
      <Card className="divide-y divide-line-soft">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="flex items-center gap-4 p-4">
            <div className="skeleton size-8 rounded-lg" />
            <div className="flex-1 space-y-2">
              <div className="skeleton h-3.5 w-32 rounded" />
              <div className="skeleton h-3 w-64 rounded" />
            </div>
            <div className="skeleton h-7 w-16 rounded-lg" />
          </div>
        ))}
      </Card>
    );
  }

  if (catalog.isError) {
    return (
      <Card className="p-5">
        <div className="flex items-start gap-3">
          <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warn" />
          <div className="min-w-0 flex-1">
            <h3 className="text-[13px] font-medium text-ink">Could not load the plugin registry</h3>
            <p className="mt-1 text-xs leading-relaxed text-muted">
              The registry is fetched from {isMise ? "mise's registry" : "asdf's plugin index"}. Check your network
              connection or add a plugin from a Git URL directly.
            </p>
            <p className="mt-2 rounded-lg border border-line bg-canvas/50 px-3 py-2 font-mono text-[11px] text-danger">
              {errorMessage(catalog.error)}
            </p>
            <div className="mt-3 flex gap-2">
              <Button size="sm" variant="secondary" onClick={() => void catalog.refetch()}>
                <RefreshCw className="size-3.5" />
                Retry
              </Button>
              <Button size="sm" variant="ghost" onClick={onAddManual}>
                <Plus className="size-3.5" />
                Add by URL
              </Button>
            </div>
          </div>
        </div>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-faint" />
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={`Search ${catalog.data?.length ?? 0} registry plugins…`}
          className="pl-9"
          aria-label="Search plugin registry"
        />
      </div>

      {rows.length === 0 ? (
        <EmptyState
          icon={<Store className="size-5" />}
          title={query ? "No registry plugins match this search" : "Registry is empty"}
          description={
            query
              ? "Try a shorter search, or add a plugin by Git URL instead."
              : `${isMise ? "mise" : "asdf"} did not return any registry entries. You can still add a plugin by URL.`
          }
          action={
            <Button size="sm" variant="secondary" onClick={onAddManual}>
              <Plus className="size-3.5" />
              Add by URL
            </Button>
          }
        />
      ) : (
        <Card className="overflow-hidden">
          <div className="divide-y divide-line-soft">
            {pageRows.map((entry) => {
              const already = installed.has(entry.name);
              return (
                <div
                  key={entry.name}
                  className="flex flex-wrap items-center gap-3 px-4 py-3 transition hover:bg-elevated/20"
                >
                  <div className="grid size-8 shrink-0 place-items-center rounded-lg border border-line bg-elevated text-muted">
                    <Package className="size-3.5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-[12px] font-medium text-ink">{entry.name}</span>
                      {already ? (
                        <Badge variant="success" className="gap-1 text-[10px]">
                          <CheckCircle2 className="size-2.5" />
                          installed
                        </Badge>
                      ) : null}
                    </div>
                    <div className="truncate text-[11px] text-faint" title={entry.url}>
                      {hostnameFromURL(entry.url)}
                      <span className="text-faint/60"> · {entry.url.replace(/^https?:\/\//, "").replace(/^www\./, "")}</span>
                    </div>
                  </div>
                  <Button
                    size="sm"
                    variant={already ? "ghost" : "secondary"}
                    className="h-7"
                    disabled={already || queue.isPending}
                    loading={pending === entry.name}
                    onClick={() => add(entry)}
                  >
                    {already ? "Added" : <><Plus className="size-3" />Add</>}
                  </Button>
                </div>
              );
            })}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line-soft px-4 py-2.5">
            <span className="text-[11px] text-faint">
              Showing {pageStart + 1}–{pageStart + pageRows.length} of {rows.length}
              {rows.length !== (catalog.data?.length ?? 0) ? ` (${catalog.data?.length ?? 0} total)` : ""}
            </span>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="ghost"
                className="h-7"
                disabled={currentPage <= 1}
                onClick={() => setPage(currentPage - 1)}
                aria-label="Previous registry page"
              >
                <ChevronLeft className="size-3.5" />
                Previous
              </Button>
              <span className="min-w-24 text-center font-mono text-[10px] text-faint">
                Page {currentPage} of {pageCount}
              </span>
              <Button
                size="sm"
                variant="ghost"
                className="h-7"
                disabled={currentPage >= pageCount}
                onClick={() => setPage(currentPage + 1)}
                aria-label="Next registry page"
              >
                Next
                <ChevronRight className="size-3.5" />
              </Button>
            </div>
          </div>
        </Card>
      )}
    </div>
  );
}

function AddPluginDialog({
  open,
  onOpenChange,
  onAdded,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded: () => void;
}) {
  const queue = useQueueOperation();
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");

  useEffect(() => {
    if (!open) return;
    setName("");
    setUrl("");
  }, [open]);

  const canSubmit = Boolean(name.trim() || url.trim()) && !queue.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add a plugin</DialogTitle>
          <DialogDescription>
            Use a registry name to resolve from the official index, or paste a Git URL. The name is inferred from the URL
            when left blank.
          </DialogDescription>
        </DialogHeader>
        <DialogBody>
          <div>
            <Label>Plugin name</Label>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="e.g. nodejs"
              className="font-mono"
              autoFocus
            />
            <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
              Optional when a URL is provided. Names containing path separators are rejected.
            </p>
          </div>
          <div>
            <Label>Git URL</Label>
            <Input
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              placeholder="https://github.com/org/tool-plugin.git"
              className="font-mono text-[12px]"
            />
            <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
              HTTPS and SSH repository URLs are supported. Leave blank to use the registry entry for the plugin name.
            </p>
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={queue.isPending}>
            Cancel
          </Button>
          <Button
            disabled={!canSubmit}
            loading={queue.isPending}
            onClick={() => {
              queue.mutate(
                { run: () => PluginsService.Add(name.trim(), url.trim()) },
                {
                  onSuccess: () => {
                    onOpenChange(false);
                    onAdded();
                  },
                },
              );
            }}
          >
            <Plus className="size-3.5" />
            Add plugin
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
