import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  Download,
  FolderGit2,
  FolderTree,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Trash2,
  X,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "../components/page-header";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import { EmptyState, InlineEmpty } from "../components/empty-state";
import { Input, Label } from "../components/ui/input";
import { useToast } from "../components/ui/toast";
import { useQueueOperation } from "../lib/actions";
import { useWorkspace, useWorkspaces, queryKeys } from "../lib/query";
import { basename, errorMessage, formatRelative, pluralize, shortenPath } from "../lib/format";
import { navigate } from "../lib/route";
import { cn } from "../lib/utils";
import {
  ProjectService,
  WorkspaceService,
  type WorkspaceConfig,
  type WorkspaceInput,
  type WorkspaceProjectView,
  type WorkspaceView,
} from "../lib/bindings";

type Draft = {
  id: string;
  name: string;
  projects: string[];
};

export function Workspaces() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const workspaces = useWorkspaces();
  const queue = useQueueOperation();

  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<WorkspaceConfig | null>(null);
  const [installingPath, setInstallingPath] = useState<string | null>(null);

  const scan = useWorkspace(draft ? null : selectedID);

  useEffect(() => {
    if (draft || selectedID) return;
    const first = workspaces.data?.[0];
    if (first) setSelectedID(first.id);
  }, [draft, selectedID, workspaces.data]);

  const save = useMutation({
    mutationFn: (input: WorkspaceInput) => WorkspaceService.Save(input),
    onSuccess: (saved) => {
      setDraft(null);
      setSelectedID(saved.id);
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces });
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspace(saved.id) });
      toast({ title: "Workspace saved", description: `${saved.name} is ready to scan.`, variant: "success" });
    },
    onError: (error) => {
      toast({ title: "Could not save workspace", description: errorMessage(error), variant: "error" });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => WorkspaceService.Delete(id),
    onSuccess: (list) => {
      setDeleteTarget(null);
      setSelectedID(list?.[0]?.id ?? null);
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces });
      toast({ title: "Workspace deleted", variant: "info" });
    },
    onError: (error) => {
      toast({ title: "Could not delete workspace", description: errorMessage(error), variant: "error" });
    },
  });

  const installAll = useMutation({
    mutationFn: (id: string) => WorkspaceService.InstallMissing(id),
    onSuccess: (operations) => {
      const count = operations?.length ?? 0;
      toast({
        title: count > 0 ? "Installs queued" : "Nothing to install",
        description:
          count > 0
            ? `${count} project ${pluralize(count, "operation")} queued behind the current run.`
            : "Every workspace project has its pinned versions installed.",
        variant: count > 0 ? "info" : "success",
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.operations });
      void queryClient.invalidateQueries({ queryKey: ["workspace"] });
    },
    onError: (error) => {
      toast({ title: "Could not queue installs", description: errorMessage(error), variant: "error" });
    },
  });

  const chooseDirectory = async () => {
    try {
      const path = await ProjectService.ChooseDirectory();
      if (!path || !draft) return;
      if (draft.projects.includes(path)) {
        toast({ title: "Project already in this workspace", variant: "warning" });
        return;
      }
      setDraft({ ...draft, projects: [...draft.projects, path] });
    } catch (error) {
      toast({ title: "Could not choose directory", description: errorMessage(error), variant: "error" });
    }
  };

  const startNew = () => {
    setSelectedID(null);
    setDraft({ id: "", name: "", projects: [] });
  };

  const startEdit = (workspace: WorkspaceConfig) => {
    setDraft({ id: workspace.id, name: workspace.name, projects: [...(workspace.projects ?? [])] });
  };

  const installProject = (project: WorkspaceProjectView) => {
    setInstallingPath(project.dir);
    queue.mutate(
      { run: () => ProjectService.InstallMissing(project.dir) },
      {
        onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["workspace"] }),
        onSettled: () => setInstallingPath(null),
      },
    );
  };

  const totalProjects = (workspaces.data ?? []).reduce((count, workspace) => count + (workspace.projects?.length ?? 0), 0);

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Workspace"
        title="Workspaces"
        description="Group related projects and scan their pinned toolchains together. Missing versions, inherited configs, and install gaps are reported per project."
        actions={
          <>
            <Button variant="ghost" size="icon" onClick={() => scan.refetch()} aria-label="Rescan workspace" disabled={!selectedID || Boolean(draft)}>
              <RefreshCw className={cn("size-4", scan.isFetching && "animate-spin")} />
            </Button>
            <Button size="sm" onClick={startNew}>
              <Plus className="size-3.5" />
              New workspace
            </Button>
          </>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[minmax(240px,300px)_minmax(0,1fr)]">
        <Card className="h-fit overflow-hidden">
          <CardHeader>
            <CardTitle>Saved workspaces</CardTitle>
            <Badge variant="outline" className="text-[10px]">
              {workspaces.data?.length ?? 0}
            </Badge>
          </CardHeader>
          <CardContent className="pt-2">
            {workspaces.isLoading ? (
              <div className="space-y-2 p-2">
                <div className="skeleton h-10 rounded-lg" />
                <div className="skeleton h-10 rounded-lg" />
              </div>
            ) : workspaces.isError ? (
              <div className="p-2">
                <InlineEmpty>{errorMessage(workspaces.error)}</InlineEmpty>
              </div>
            ) : (workspaces.data?.length ?? 0) === 0 ? (
              <div className="px-2 pb-2">
                <p className="py-3 text-[11px] leading-relaxed text-faint">
                  Create a workspace to keep related repositories together and check their toolchains in one pass.
                </p>
                <Button size="sm" variant="secondary" className="w-full" onClick={startNew}>
                  <Plus className="size-3.5" />
                  New workspace
                </Button>
              </div>
            ) : (
              <div className="-mx-1 divide-y divide-line-soft">
                {(workspaces.data ?? []).map((workspace) => {
                  const active = workspace.id === selectedID && !draft;
                  return (
                    <button
                      key={workspace.id}
                      onClick={() => {
                        setDraft(null);
                        setSelectedID(workspace.id);
                      }}
                      className={cn(
                        "flex w-full items-center gap-2.5 rounded-md px-2 py-2.5 text-left transition",
                        active ? "bg-elevated/60" : "hover:bg-elevated/40",
                      )}
                    >
                      <FolderTree className={cn("size-3.5 shrink-0", active ? "text-accent" : "text-faint")} />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-[12px] text-ink">{workspace.name}</div>
                        <div className="mt-0.5 text-[10px] text-faint">
                          {workspace.projects?.length ?? 0} {pluralize(workspace.projects?.length ?? 0, "project")}
                        </div>
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
            <div className="mt-3 border-t border-line-soft px-2 pt-3 text-[10px] leading-relaxed text-faint">
              {workspaces.data?.length ?? 0} {pluralize(workspaces.data?.length ?? 0, "workspace")} · {totalProjects}{" "}
              {pluralize(totalProjects, "project")}
            </div>
          </CardContent>
        </Card>

        <div className="min-w-0 space-y-4">
          {draft ? (
            <WorkspaceEditor
              draft={draft}
              onChange={setDraft}
              onAddProject={chooseDirectory}
              onRemoveProject={(path) =>
                setDraft({ ...draft, projects: draft.projects.filter((project) => project !== path) })
              }
              onCancel={() => {
                setDraft(null);
                if (!selectedID) setSelectedID(workspaces.data?.[0]?.id ?? null);
              }}
              onSave={() => save.mutate(draft)}
              saving={save.isPending}
            />
          ) : scan.isLoading ? (
            <Card className="p-5">
              <div className="space-y-3">
                <div className="skeleton h-8 w-48 rounded" />
                <div className="grid gap-3 sm:grid-cols-4">
                  {Array.from({ length: 4 }).map((_, index) => (
                    <div key={index} className="skeleton h-16 rounded-lg" />
                  ))}
                </div>
                <div className="skeleton h-40 rounded-xl" />
              </div>
            </Card>
          ) : scan.isError ? (
            <InlineEmpty>{errorMessage(scan.error)}</InlineEmpty>
          ) : scan.data ? (
            <WorkspaceDetail
              view={scan.data}
              onRefresh={() => scan.refetch()}
              refreshing={scan.isFetching}
              onEdit={() => {
                const workspace = workspaces.data?.find((item) => item.id === scan.data?.id);
                if (workspace) startEdit(workspace);
              }}
              onDelete={() => {
                const workspace = workspaces.data?.find((item) => item.id === scan.data?.id);
                if (workspace) setDeleteTarget(workspace);
              }}
              onInstallAll={() => installAll.mutate(scan.data.id)}
              installing={installAll.isPending}
              onInstallProject={installProject}
              installingProject={installingPath}
            />
          ) : (
            <Card>
              <EmptyState
                icon={<FolderTree className="size-5" />}
                title="Choose or create a workspace"
                description="A workspace scans several project directories and shows pinned tools, missing versions, and config inheritance side by side."
                action={
                  <Button size="sm" onClick={startNew}>
                    <Plus className="size-3.5" />
                    New workspace
                  </Button>
                }
              />
            </Card>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title={`Delete ${deleteTarget?.name ?? "workspace"}?`}
        description="The workspace grouping is removed. Project directories and their config files are not touched."
        confirmLabel="Delete workspace"
        destructive
        loading={remove.isPending}
        onConfirm={() => {
          if (deleteTarget) remove.mutate(deleteTarget.id);
        }}
      />
    </div>
  );
}

function WorkspaceEditor({
  draft,
  onChange,
  onAddProject,
  onRemoveProject,
  onCancel,
  onSave,
  saving,
}: {
  draft: Draft;
  onChange: (draft: Draft) => void;
  onAddProject: () => void;
  onRemoveProject: (path: string) => void;
  onCancel: () => void;
  onSave: () => void;
  saving: boolean;
}) {
  const canSave = draft.name.trim().length > 0 && draft.projects.length > 0 && !saving;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{draft.id ? "Edit workspace" : "New workspace"}</CardTitle>
        <Badge variant="outline" className="text-[10px]">
          {draft.projects.length} {pluralize(draft.projects.length, "project")}
        </Badge>
      </CardHeader>
      <CardContent className="space-y-4 pt-4">
        <div>
          <Label>Workspace name</Label>
          <Input
            value={draft.name}
            onChange={(event) => onChange({ ...draft, name: event.target.value })}
            placeholder="e.g. Platform services"
            autoFocus
          />
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between gap-2">
            <Label className="mb-0">Projects</Label>
            <Button size="sm" variant="secondary" className="h-7" onClick={onAddProject}>
              <Plus className="size-3.5" />
              Add project
            </Button>
          </div>
          {draft.projects.length === 0 ? (
            <div className="rounded-lg border border-dashed border-line px-4 py-6 text-center text-[11px] leading-relaxed text-faint">
              Add at least one project directory. Each one is scanned with its nearest
              <span className="font-mono"> mise.toml</span> or <span className="font-mono">.tool-versions</span>.
            </div>
          ) : (
            <div className="divide-y divide-line-soft overflow-hidden rounded-lg border border-line-soft">
              {draft.projects.map((path) => (
                <div key={path} className="flex items-center gap-2.5 px-3 py-2">
                  <FolderGit2 className="size-3.5 shrink-0 text-faint" />
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[12px] text-ink">{basename(path)}</div>
                    <div className="truncate font-mono text-[10px] text-faint" title={path}>
                      {shortenPath(path, 64)}
                    </div>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => onRemoveProject(path)}
                    aria-label={`Remove ${path}`}
                    className="text-faint hover:text-danger"
                  >
                    <X className="size-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-line-soft pt-3">
          <Button variant="ghost" size="sm" onClick={onCancel} disabled={saving}>
            Cancel
          </Button>
          <Button size="sm" disabled={!canSave} loading={saving} onClick={onSave}>
            <Save className="size-3.5" />
            Save workspace
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function WorkspaceDetail({
  view,
  onRefresh,
  refreshing,
  onEdit,
  onDelete,
  onInstallAll,
  installing,
  onInstallProject,
  installingProject,
}: {
  view: WorkspaceView;
  onRefresh: () => void;
  refreshing: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onInstallAll: () => void;
  installing: boolean;
  onInstallProject: (project: WorkspaceProjectView) => void;
  installingProject: string | null;
}) {
  const projects = view.projects ?? [];
  const missing = view.missing ?? [];
  const missingProjects = view.missingProjects ?? 0;

  return (
    <>
      <Card>
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-line-soft px-5 py-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <FolderTree className="size-4 text-accent" />
              <h2 className="truncate text-[15px] font-semibold text-ink">{view.name}</h2>
              {missingProjects > 0 ? (
                <Badge variant="danger" className="text-[10px]">
                  {missingProjects} {pluralize(missingProjects, "project")} missing versions
                </Badge>
              ) : (
                <Badge variant="success" className="text-[10px]">
                  all installed
                </Badge>
              )}
            </div>
            <p className="mt-1 text-[11px] text-faint">Scanned {formatRelative(view.scannedAt)}</p>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <Button variant="ghost" size="icon-sm" onClick={onRefresh} aria-label="Rescan workspace">
              <RefreshCw className={cn("size-3.5", refreshing && "animate-spin")} />
            </Button>
            <Button variant="ghost" size="icon-sm" onClick={onEdit} aria-label="Edit workspace">
              <Pencil className="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={onDelete}
              aria-label="Delete workspace"
              className="text-faint hover:text-danger"
            >
              <Trash2 className="size-3.5" />
            </Button>
            <Button
              size="sm"
              className="h-7"
              disabled={missingProjects === 0 || installing}
              loading={installing}
              onClick={onInstallAll}
            >
              <Download className="size-3.5" />
              Install missing
            </Button>
          </div>
        </div>
        <CardContent className="pt-4">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <Metric label="Projects" value={String(projects.length)} />
            <Metric label="Pinned tools" value={String(view.pinnedCount ?? 0)} />
            <Metric label="Missing versions" value={String(missing.length)} tone={missing.length > 0 ? "warn" : undefined} />
            <Metric label="Healthy projects" value={String(Math.max(0, projects.length - missingProjects))} />
          </div>
          {missing.length > 0 ? (
            <div className="mt-4 flex flex-wrap items-center gap-1.5 rounded-lg border border-danger/20 bg-danger/5 px-3 py-2">
              <AlertTriangle className="size-3.5 shrink-0 text-danger" />
              <span className="text-[11px] text-danger">Missing across this workspace:</span>
              {missing.map((tool) => (
                <span key={tool} className="rounded bg-danger/10 px-1.5 py-0.5 font-mono text-[10px] text-danger">
                  {tool}
                </span>
              ))}
            </div>
          ) : null}
        </CardContent>
      </Card>

      {projects.length === 0 ? (
        <Card>
          <EmptyState
            icon={<FolderGit2 className="size-5" />}
            title="No projects in this workspace"
            description="Edit the workspace and add project directories to start scanning them."
            action={
              <Button size="sm" variant="secondary" onClick={onEdit}>
                <Pencil className="size-3.5" />
                Edit workspace
              </Button>
            }
          />
        </Card>
      ) : (
        <div className="space-y-3">
          {projects.map((project) => (
            <WorkspaceProjectCard
              key={project.dir}
              project={project}
              onInstall={() => onInstallProject(project)}
              installing={installingProject === project.dir}
            />
          ))}
        </div>
      )}
    </>
  );
}

function WorkspaceProjectCard({
  project,
  onInstall,
  installing,
}: {
  project: WorkspaceProjectView;
  onInstall: () => void;
  installing: boolean;
}) {
  const currentMap = useMemo(
    () => new Map((project.current ?? []).map((item) => [item.name, item])),
    [project.current],
  );
  const entries = project.entries ?? [];
  const missing = project.missing ?? [];
  const warnings = project.warnings ?? [];
  const configLabel = project.configFile ? basename(project.configFile) : project.engine === "mise" ? "mise.toml" : ".tool-versions";

  return (
    <Card className={cn("overflow-hidden", !project.exists && "border-warn/25")}>
      <div className="flex flex-wrap items-center gap-2.5 border-b border-line-soft bg-canvas/25 px-4 py-3">
        <FolderGit2 className={cn("size-4 shrink-0", project.exists ? "text-muted" : "text-warn")} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-[12px] font-medium text-ink">{project.name}</span>
            {project.usingParent ? (
              <Badge variant="warning" className="text-[9px]">parent config</Badge>
            ) : project.fileInProject ? (
              <Badge variant="success" className="text-[9px]">local file</Badge>
            ) : project.fileExists ? (
              <Badge variant="outline" className="text-[9px]">inherited</Badge>
            ) : (
              <Badge variant="outline" className="text-[9px]">no config</Badge>
            )}
            {missing.length > 0 ? (
              <Badge variant="danger" className="text-[9px]">
                {missing.length} missing
              </Badge>
            ) : null}
          </div>
          <div className="mt-0.5 truncate font-mono text-[10px] text-faint" title={project.dir}>
            {shortenPath(project.dir, 70)}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-[11px]"
            onClick={() => navigate("/project", { dir: project.dir })}
            disabled={!project.exists}
          >
            Open
          </Button>
          <Button
            variant="secondary"
            size="sm"
            className="h-7"
            onClick={onInstall}
            disabled={!project.exists || missing.length === 0 || installing}
            loading={installing}
          >
            <Download className="size-3" />
            Install
          </Button>
        </div>
      </div>

      <CardContent className="pt-3">
        {!project.exists ? (
          <div className="flex items-start gap-2 text-[11px] leading-relaxed text-warn">
            <AlertTriangle className="mt-0.5 size-3 shrink-0" />
            <span>{warnings.join(" ") || "Project directory is missing."}</span>
          </div>
        ) : entries.length === 0 ? (
          <p className="text-[11px] leading-relaxed text-faint">
            No tools pinned in {configLabel}. {project.usingParent ? "This project inherits a parent config." : ""}
          </p>
        ) : (
          <div className="overflow-hidden rounded-lg border border-line-soft">
            {entries.map((entry) => {
              const current = currentMap.get(entry.tool);
              return (
                <div
                  key={entry.tool}
                  className="flex items-center gap-3 border-b border-line-soft px-3 py-2 last:border-b-0"
                >
                  <span className="w-28 shrink-0 truncate font-mono text-[11px] text-ink">{entry.tool}</span>
                  <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted">
                    {(entry.versions ?? []).join(" ")}
                  </span>
                  {current ? (
                    current.system ? (
                      <Badge variant="info" className="text-[9px]">system</Badge>
                    ) : current.installed ? (
                      <Badge variant="success" className="text-[9px]">active</Badge>
                    ) : (
                      <Badge variant="danger" className="text-[9px]">missing</Badge>
                    )
                  ) : (
                    <Badge variant="outline" className="text-[9px]">unresolved</Badge>
                  )}
                </div>
              );
            })}
          </div>
        )}

        {warnings.length > 0 && project.exists && !project.usingParent ? (
          <div className="mt-2.5 space-y-1">
            {warnings.map((warning) => (
              <div key={warning} className="flex items-start gap-2 text-[10px] leading-relaxed text-warn">
                <AlertTriangle className="mt-0.5 size-2.5 shrink-0" />
                <span>{warning}</span>
              </div>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: "warn" }) {
  return (
    <div className="rounded-lg border border-line-soft bg-canvas/30 px-3 py-2.5">
      <div className="text-[9px] font-semibold uppercase tracking-[0.13em] text-faint">{label}</div>
      <div className={cn("mt-1 font-mono text-[18px] leading-none text-ink", tone === "warn" && "text-warn")}>{value}</div>
    </div>
  );
}
