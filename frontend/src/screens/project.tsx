import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  Download,
  ExternalLink,
  FileCode2,
  FolderGit2,
  FolderOpen,
  History,
  Plus,
  RefreshCw,
  Save,
  X,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "../components/page-header";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState, InlineEmpty } from "../components/empty-state";
import { Input, Label } from "../components/ui/input";
import { Hint } from "../components/ui/tooltip";
import { useToast } from "../components/ui/toast";
import { useProject, useStatus, useTools, queryKeys } from "../lib/query";
import { useQueueOperation } from "../lib/actions";
import { basename, errorMessage, pluralize, shortenPath, splitVersions } from "../lib/format";
import { navigate } from "../lib/route";
import { cn } from "../lib/utils";
import {
  ManagerService,
  ProjectService,
  type ProjectView,
  type ToolVersionEntry,
} from "../lib/bindings";

export function Project() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const status = useStatus();
  const tools = useTools();
  const queue = useQueueOperation();

  const [dir, setDir] = useState("");
  const [entries, setEntries] = useState<ToolVersionEntry[]>([]);
  const [dirty, setDirty] = useState(false);
  const [addTool, setAddTool] = useState("");
  const [addVersions, setAddVersions] = useState("");
  const [loadedKey, setLoadedKey] = useState("");

  const project = useProject(dir, false, status.isSuccess);

  useEffect(() => {
    if (dirty || !project.data) return;
    const key = `${project.data.dir}:${project.data.loadedAt}`;
    if (key === loadedKey) return;
    setLoadedKey(key);
    setEntries(normalizeEntries(project.data.entries ?? []));
  }, [project.data, dirty, loadedKey]);

  const save = useMutation({
    mutationFn: (next: ToolVersionEntry[]) =>
      ProjectService.Save(project.data?.dir ?? dir, next),
    onSuccess: (view) => {
      queryClient.setQueryData(queryKeys.project(dir), view);
      setEntries(normalizeEntries(view.entries ?? []));
      setDirty(false);
      void queryClient.invalidateQueries({ queryKey: queryKeys.recent });
      void queryClient.invalidateQueries({ queryKey: queryKeys.tools });
      void queryClient.invalidateQueries({ queryKey: queryKeys.status });
      toast({ title: "Project saved", description: `${view.entries?.length ?? 0} tools pinned in ${basename(view.filePath)}.`, variant: "success" });
    },
    onError: (error) => {
      toast({ title: "Could not save .tool-versions", description: errorMessage(error), variant: "error" });
    },
  });

  const recent = project.data?.recent ?? [];
  const plugins = project.data?.plugins ?? [];
  const currentMap = useMemo(
    () => new Map((project.data?.current ?? []).map((item) => [item.name, item])),
    [project.data],
  );
  const toolNames = useMemo(() => {
    const names = new Set<string>();
    for (const plugin of plugins) names.add(plugin);
    for (const tool of tools.data ?? []) names.add(tool.name);
    for (const entry of entries) names.add(entry.tool);
    return [...names].sort((left, right) => left.localeCompare(right));
  }, [plugins, tools.data, entries]);

  const chooseDirectory = async () => {
    try {
      const path = await ProjectService.ChooseDirectory();
      if (!path) return;
      setDirty(false);
      setDir(path);
      void ProjectService.Open(path, true)
        .then(() => {
          void queryClient.invalidateQueries({ queryKey: queryKeys.recent });
        })
        .catch(() => undefined);
    } catch (error) {
      toast({ title: "Could not open folder picker", description: errorMessage(error), variant: "error" });
    }
  };

  const openRecent = (path: string) => {
    if (dirty) {
      const confirmed = window.confirm("Discard unsaved .tool-versions changes?");
      if (!confirmed) return;
    }
    setDirty(false);
    setDir(path);
    void ProjectService.Open(path, true)
      .then(() => {
        void queryClient.invalidateQueries({ queryKey: queryKeys.recent });
      })
      .catch(() => undefined);
  };

  const refresh = () => {
    if (dirty && !window.confirm("Discard unsaved .tool-versions changes and reload?")) return;
    setDirty(false);
    void queryClient.invalidateQueries({ queryKey: queryKeys.project(dir) });
  };

  const browse = () => {
    if (project.data?.dir) void ProjectService.Reveal(project.data.dir);
  };

  const updateVersions = (index: number, value: string) => {
    setEntries((current) =>
      current.map((entry, entryIndex) =>
        entryIndex === index ? { ...entry, versions: splitVersions(value) } : entry,
      ),
    );
    setDirty(true);
  };

  const removeEntry = (index: number) => {
    setEntries((current) => current.filter((_, entryIndex) => entryIndex !== index));
    setDirty(true);
  };

  const addEntry = () => {
    const tool = addTool.trim();
    const versions = splitVersions(addVersions);
    if (!tool) {
      toast({ title: "Name the tool first", variant: "warning" });
      return;
    }
    if (entries.some((entry) => entry.tool === tool)) {
      toast({ title: `${tool} is already pinned`, description: "Edit the existing row instead.", variant: "warning" });
      return;
    }
    setEntries((current) => [...current, { tool, versions, line: 0 }]);
    setAddTool("");
    setAddVersions("");
    setDirty(true);
  };

  const invalidEntries = entries.filter((entry) => (entry.versions?.length ?? 0) === 0);
  const duplicateTools = findDuplicates(entries.map((entry) => entry.tool));
  const canSave = dirty && entries.length > 0 && invalidEntries.length === 0 && duplicateTools.length === 0;
  const fileLabel = project.data?.filePath ? basename(project.data.filePath) : ".tool-versions";

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Workspace"
        title="Project"
        description="Inspect and edit the .tool-versions file for any project. Comments and unrelated lines are preserved; only pinned entries change."
        actions={
          <>
            <Hint label="Open the project directory">
              <Button variant="ghost" size="icon" onClick={browse} aria-label="Reveal project directory" disabled={!project.data?.dir}>
                <FolderOpen className="size-4" />
              </Button>
            </Hint>
            <Hint label="Reload from disk">
              <Button variant="ghost" size="icon" onClick={refresh} aria-label="Reload project">
                <RefreshCw className={cn("size-4", project.isFetching && "animate-spin")} />
              </Button>
            </Hint>
            <Button variant="secondary" size="sm" onClick={chooseDirectory}>
              <FolderGit2 className="size-3.5" />
              Open folder
            </Button>
            {project.data?.dir ? (
              <Button
                size="sm"
                disabled={queue.isPending || (project.data?.missing?.length ?? 0) === 0}
                loading={queue.isPending}
                onClick={() =>
                  queue.mutate(
                    { run: () => ProjectService.InstallMissing(project.data?.dir ?? "") },
                    {
                      onSuccess: () =>
                        void queryClient.invalidateQueries({ queryKey: queryKeys.project(dir) }),
                    },
                  )
                }
              >
                <Download className="size-3.5" />
                Install missing
                {(project.data?.missing?.length ?? 0) > 0 ? (
                  <span className="rounded-full bg-accent-ink/15 px-1.5 text-[10px] font-semibold">
                    {project.data?.missing?.length}
                  </span>
                ) : null}
              </Button>
            ) : null}
          </>
        }
      />

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center gap-3 border-b border-line-soft bg-canvas/25 px-4 py-3">
          <div className="flex min-w-0 flex-1 items-center gap-2 rounded-lg border border-line bg-canvas/50 px-3 py-1.5">
            <span className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.12em] text-faint">Directory</span>
            <span className="min-w-0 truncate font-mono text-[11px] text-ink" title={project.data?.dir}>
              {project.data?.dir || "resolving…"}
            </span>
          </div>
          {project.data?.usingParent ? (
            <Badge variant="warning" className="gap-1">
              <AlertTriangle className="size-3" />
              parent file
            </Badge>
          ) : project.data?.fileInProject ? (
            <Badge variant="success" className="gap-1">
              <CheckCircle2 className="size-3" />
              local file
            </Badge>
          ) : project.data?.fileExists ? (
            <Badge variant="outline">inherited</Badge>
          ) : (
            <Badge variant="outline">no file yet</Badge>
          )}
          <span className="truncate font-mono text-[10px] text-faint" title={project.data?.filePath}>
            {project.data?.filePath ? shortenPath(project.data.filePath, 50) : ""}
          </span>
        </div>

        {project.isError ? (
          <div className="p-4">
            <InlineEmpty>{errorMessage(project.error)}</InlineEmpty>
          </div>
        ) : project.isLoading ? (
          <div className="space-y-2 p-4">
            <div className="skeleton h-10 rounded-lg" />
            <div className="skeleton h-10 rounded-lg" />
            <div className="skeleton h-10 rounded-lg" />
          </div>
        ) : (
          <div className="grid gap-0 lg:grid-cols-[minmax(0,1.6fr)_minmax(290px,1fr)]">
            <div className="min-w-0 border-b border-line-soft p-4 lg:border-b-0 lg:border-r">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <FileCode2 className="size-3.5 text-faint" />
                  <span className="font-mono text-[11px] text-muted">{fileLabel}</span>
                  {dirty ? <Badge variant="warning" className="text-[10px]">unsaved</Badge> : null}
                </div>
                <span className="text-[10px] text-faint">
                  {entries.length} {pluralize(entries.length, "tool")} pinned
                </span>
              </div>

              {entries.length === 0 ? (
                <EmptyState
                  icon={<FileCode2 className="size-5" />}
                  title={project.data?.fileExists ? "No tools pinned in this file" : "Start a .tool-versions file"}
                  description={
                    project.data?.usingParent
                      ? "This project inherits its file from a parent directory. Saving here creates a project-local file that takes precedence."
                      : "Add a tool below, then save. The file is written atomically next to your sources."
                  }
                  className="py-8"
                />
              ) : (
                <div className="space-y-2">
                  {entries.map((entry, index) => {
                    const current = currentMap.get(entry.tool);
                    const installed = current?.installed ?? false;
                    return (
                      <div
                        key={`${entry.tool}-${index}`}
                        className={cn(
                          "group rounded-lg border px-3 py-2.5 transition",
                          (entry.versions?.length ?? 0) === 0
                            ? "border-danger/35 bg-danger/5"
                            : "border-line-soft bg-canvas/35 hover:border-line",
                        )}
                      >
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="w-32 shrink-0 truncate font-mono text-[12px] font-medium text-ink">
                            {entry.tool}
                          </span>
                          <Input
                            value={(entry.versions ?? []).join(" ")}
                            onChange={(event) => updateVersions(index, event.target.value)}
                            className="h-7 min-w-40 flex-1 font-mono text-[11px]"
                            placeholder="version or ref:…"
                            aria-label={`Versions for ${entry.tool}`}
                          />
                          <div className="flex shrink-0 items-center gap-1">
                            {current ? (
                              current.system ? (
                                <Badge variant="info" className="text-[10px]">system</Badge>
                              ) : installed ? (
                                <Badge variant="success" className="text-[10px]">active</Badge>
                              ) : (
                                <Badge variant="danger" className="text-[10px]">missing</Badge>
                              )
                            ) : (
                              <Badge variant="outline" className="text-[10px]">unresolved</Badge>
                            )}
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => removeEntry(index)}
                              aria-label={`Remove ${entry.tool}`}
                              className="text-faint hover:text-danger"
                            >
                              <X className="size-3.5" />
                            </Button>
                          </div>
                        </div>
                        {(entry.versions?.length ?? 0) > 1 ? (
                          <p className="mt-1.5 pl-32 text-[10px] text-faint">
                            Fallback order: {(entry.versions ?? []).join(" → ")}
                          </p>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              )}

              <div className="mt-4 rounded-lg border border-dashed border-line p-3">
                <Label className="mb-2">Add a tool</Label>
                <div className="flex flex-wrap items-center gap-2">
                  <Input
                    value={addTool}
                    onChange={(event) => setAddTool(event.target.value)}
                    placeholder="tool name"
                    list="project-tool-names"
                    className="h-8 min-w-32 flex-1 font-mono text-[11px]"
                  />
                  <datalist id="project-tool-names">
                    {toolNames.map((name) => (
                      <option key={name} value={name} />
                    ))}
                  </datalist>
                  <Input
                    value={addVersions}
                    onChange={(event) => setAddVersions(event.target.value)}
                    placeholder="1.2.3 (space-separated fallbacks allowed)"
                    className="h-8 min-w-44 flex-[2] font-mono text-[11px]"
                    onKeyDown={(event) => {
                      if (event.key === "Enter") addEntry();
                    }}
                  />
                  <Button size="sm" variant="secondary" className="h-8" onClick={addEntry}>
                    <Plus className="size-3.5" />
                    Add
                  </Button>
                </div>
                <p className="mt-2 text-[10px] leading-relaxed text-faint">
                  {plugins.length > 0
                    ? `Installed plugins: ${plugins.slice(0, 8).join(", ")}${plugins.length > 8 ? "…" : ""}`
                    : "No plugins installed yet. Add one on the Plugins screen so asdf can resolve versions."}
                </p>
              </div>

              {invalidEntries.length > 0 ? (
                <div className="mt-3 flex items-start gap-2 text-[11px] leading-relaxed text-danger">
                  <AlertTriangle className="mt-0.5 size-3 shrink-0" />
                  <span>
                    {invalidEntries.map((entry) => entry.tool).join(", ")} {invalidEntries.length === 1 ? "has" : "have"} no
                    version. Add a version or remove the row before saving.
                  </span>
                </div>
              ) : null}
              {duplicateTools.length > 0 ? (
                <div className="mt-3 flex items-start gap-2 text-[11px] leading-relaxed text-danger">
                  <AlertTriangle className="mt-0.5 size-3 shrink-0" />
                  <span>Duplicate tools: {duplicateTools.join(", ")}. Each tool may appear once.</span>
                </div>
              ) : null}

              <div className="mt-4 flex items-center justify-between gap-3 border-t border-line-soft pt-3">
                <p className="text-[10px] leading-relaxed text-faint">
                  Saving writes <span className="font-mono">{fileLabel}</span> atomically. Comments are preserved.
                </p>
                <div className="flex shrink-0 items-center gap-2">
                  {dirty ? (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        setEntries(normalizeEntries(project.data?.entries ?? []));
                        setDirty(false);
                      }}
                    >
                      Revert
                    </Button>
                  ) : null}
                  <Button size="sm" disabled={!canSave} loading={save.isPending} onClick={() => save.mutate(entries)}>
                    <Save className="size-3.5" />
                    Save file
                  </Button>
                </div>
              </div>
            </div>

            <div className="space-y-4 p-4">
              <ProjectWarnings view={project.data} />
              <RecentProjects
                recent={recent}
                activeDir={project.data?.dir ?? ""}
                onOpen={openRecent}
              />
              <GlobalFile view={project.data} />
            </div>
          </div>
        )}
      </Card>

      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-line-soft bg-canvas/30 px-4 py-3">
        <p className="text-[11px] leading-relaxed text-faint">
          The effective file is the nearest <span className="font-mono">.tool-versions</span> walking up from the project
          directory. Saving always targets the selected project itself, never a parent.
        </p>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 text-[11px]"
          onClick={() => void ManagerService.OpenURL("https://asdf-vm.com/manage/configuration.html#tool-versions")}
        >
          .tool-versions docs
          <ExternalLink className="size-3" />
        </Button>
      </div>
    </div>
  );
}

function ProjectWarnings({ view }: { view?: ProjectView }) {
  const warnings = view?.warnings ?? [];
  const missing = view?.missing ?? [];

  if (warnings.length === 0 && missing.length === 0) {
    return (
      <Card className="border-accent/20 bg-accent/[0.04]">
        <CardContent className="flex items-start gap-3 pt-4">
          <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-accent" />
          <div>
            <p className="text-[12px] font-medium text-ink">This project is ready</p>
            <p className="mt-0.5 text-[11px] leading-relaxed text-muted">
              Every pinned tool resolves to an installed version.
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Attention</CardTitle>
        {missing.length > 0 ? <Badge variant="danger">{missing.length} missing</Badge> : null}
      </CardHeader>
      <CardContent className="space-y-2.5 pt-3">
        {missing.length > 0 ? (
          <div className="rounded-lg border border-danger/20 bg-danger/5 px-3 py-2">
            <p className="text-[11px] font-medium text-danger">Missing versions</p>
            <p className="mt-0.5 font-mono text-[11px] text-danger/80">{missing.join(", ")}</p>
          </div>
        ) : null}
        {warnings.map((warning) => (
          <div key={warning} className="flex items-start gap-2 text-[11px] leading-relaxed text-warn">
            <AlertTriangle className="mt-0.5 size-3 shrink-0" />
            <span>{warning}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function RecentProjects({
  recent,
  activeDir,
  onOpen,
}: {
  recent: NonNullable<ProjectView["recent"]>;
  activeDir: string;
  onOpen: (path: string) => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent projects</CardTitle>
        <History className="size-3.5 text-faint" />
      </CardHeader>
      <CardContent className="pt-2">
        {recent.length === 0 ? (
          <p className="py-2 text-[11px] leading-relaxed text-faint">
            Projects appear here after you open them. Set a default project in Settings to skip this step.
          </p>
        ) : (
          <div className="-mx-1 divide-y divide-line-soft">
            {recent.slice(0, 7).map((entry) => {
              const active = sameDir(entry.path, activeDir);
              return (
                <button
                  key={entry.path}
                  onClick={() => onOpen(entry.path)}
                  disabled={!entry.exists}
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-md px-2 py-2 text-left transition",
                    active ? "bg-elevated/60" : "hover:bg-elevated/40",
                    !entry.exists && "opacity-45",
                  )}
                >
                  <FolderGit2 className={cn("size-3.5 shrink-0", active ? "text-accent" : "text-faint")} />
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[12px] text-ink">{entry.name || basename(entry.path)}</div>
                    <div className="truncate font-mono text-[10px] text-faint" title={entry.path}>
                      {shortenPath(entry.path, 38)}
                    </div>
                  </div>
                  {entry.hasToolVersions ? (
                    <Badge variant="outline" className="shrink-0 px-1.5 text-[9px] font-mono">
                      .tool-versions
                    </Badge>
                  ) : null}
                </button>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function GlobalFile({ view }: { view?: ProjectView }) {
  const entries = view?.globalEntries ?? [];
  return (
    <Card>
      <CardHeader>
        <CardTitle>Global pins</CardTitle>
        <Badge variant="outline" className="text-[10px]">
          {entries.length} {pluralize(entries.length, "tool")}
        </Badge>
      </CardHeader>
      <CardContent className="pt-3">
        <div className="mb-2 truncate font-mono text-[10px] text-faint" title={view?.globalFilePath}>
          {view?.globalFilePath ? shortenPath(view.globalFilePath, 42) : "~/.tool-versions"}
        </div>
        {entries.length === 0 ? (
          <p className="text-[11px] leading-relaxed text-faint">
            No global pins. Use the pin action on the Tools screen to set a fallback for every project.
          </p>
        ) : (
          <div className="space-y-1.5">
            {entries.map((entry) => (
              <div key={entry.tool} className="flex items-center gap-2 text-[11px]">
                <span className="w-24 shrink-0 truncate font-mono text-muted">{entry.tool}</span>
                <span className="min-w-0 flex-1 truncate font-mono text-faint">
                  {(entry.versions ?? []).join(" ")}
                </span>
              </div>
            ))}
          </div>
        )}
        <button
          onClick={() => navigate("/tools")}
          className="mt-3 flex w-full items-center justify-center gap-1.5 rounded-md border border-line-soft py-1.5 text-[11px] text-muted transition hover:border-line hover:text-ink"
        >
          Manage versions in Tools
          <ArrowRight className="size-3" />
        </button>
      </CardContent>
    </Card>
  );
}

function normalizeEntries(entries: ToolVersionEntry[]): ToolVersionEntry[] {
  return entries.map((entry) => ({
    tool: entry.tool,
    versions: [...(entry.versions ?? [])],
    line: entry.line,
  }));
}

function findDuplicates(values: string[]): string[] {
  const seen = new Set<string>();
  const duplicates = new Set<string>();
  for (const value of values) {
    if (seen.has(value)) duplicates.add(value);
    seen.add(value);
  }
  return [...duplicates];
}

function sameDir(left: string, right: string): boolean {
  return left.replace(/\/+$/, "") === right.replace(/\/+$/, "");
}
