import { useEffect, useRef, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  Database,
  Download,
  ExternalLink,
  FileCog,
  FolderOpen,
  Info,
  Monitor,
  Moon,
  RefreshCw,
  RotateCcw,
  Save,
  Settings2,
  ShieldCheck,
  Sparkles,
  Sun,
  Terminal,
  XCircle,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "../components/page-header";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input, Label } from "../components/ui/input";
import { ConfirmDialog } from "../components/ui/confirm-dialog";
import { useToast } from "../components/ui/toast";
import {
  queryKeys,
  useCheckForUpdates,
  useDiagnostics,
  useInstallUpdate,
  useInvalidateAppData,
  useSettings,
  useStatus,
  useUpdateStatus,
  useWSLDistros,
} from "../lib/query";
import { errorMessage, formatBytes, shortenPath, statusLabel } from "../lib/format";
import { applyTheme, normalizeTheme, readStoredTheme, type Theme } from "../lib/theme";
import { cn } from "../lib/utils";
import {
  ManagerService,
  SettingsService,
  type ManagerStatus,
  type Settings as SettingsModel,
} from "../lib/bindings";

const THEMES: { value: Theme; label: string; icon: typeof Sun; hint: string }[] = [
  { value: "system", label: "System", icon: Monitor, hint: "Follow the desktop appearance" },
  { value: "dark", label: "Dark", icon: Moon, hint: "Always use the dark palette" },
  { value: "light", label: "Light", icon: Sun, hint: "Always use the light palette" },
];

const ENGINES: { value: "auto" | "asdf" | "mise"; label: string; hint: string }[] = [
  { value: "auto", label: "Auto", hint: "Prefer asdf, then mise" },
  { value: "asdf", label: "asdf", hint: "Drive the asdf CLI" },
  { value: "mise", label: "mise", hint: "Drive the mise CLI" },
];

const RUNNERS: { value: "auto" | "native" | "wsl"; label: string; hint: string }[] = [
  { value: "auto", label: "Auto", hint: "Native first, then WSL2" },
  { value: "native", label: "Native", hint: "Run on Windows" },
  { value: "wsl", label: "WSL2", hint: "Run inside a distribution" },
];

const REPO_URL = "https://github.com/AshutoshSajan/asdf-mise-gui";
const RELEASES_URL = `${REPO_URL}/releases`;

export function Settings() {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateAppData();
  const { toast } = useToast();

  const status = useStatus();
  const settings = useSettings();
  const wslDistros = useWSLDistros(status.data?.os === "windows");
  const [diagnosticsEnabled, setDiagnosticsEnabled] = useState(false);
  const diagnostics = useDiagnostics(diagnosticsEnabled);
  const updateStatus = useUpdateStatus();
  const checkUpdate = useCheckForUpdates();
  const installUpdate = useInstallUpdate();

  const [form, setForm] = useState<SettingsModel>({
    engine: "auto",
    binaryPath: "",
    dataDir: "",
    defaultProjectDir: "",
    theme: readStoredTheme(),
    checkForUpdates: true,
    recentProjects: [],
    workspaces: [],
    runnerMode: "auto",
    wslDistro: "",
  });
  const [dirty, setDirty] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const savedTheme = useRef<Theme>(normalizeTheme(settings.data?.settings.theme ?? readStoredTheme()));

  useEffect(() => {
    if (dirty || !settings.data) return;
    setForm(settings.data.settings);
  }, [settings.data, dirty]);

  useEffect(() => {
    savedTheme.current = normalizeTheme(settings.data?.settings.theme ?? readStoredTheme());
  }, [settings.data?.settings.theme]);

  // A theme card is a live preview. Navigating away without saving restores
  // the persisted choice instead of leaving the next screen in preview mode.
  useEffect(() => {
    return () => {
      applyTheme(savedTheme.current, { persist: false });
    };
  }, []);

  const save = useMutation({
    mutationFn: (next: SettingsModel) => SettingsService.Save(next),
    onSuccess: (view) => {
      const theme = normalizeTheme(view.settings.theme);
      applyTheme(theme);
      queryClient.setQueryData(queryKeys.settings, view);
      queryClient.setQueryData<ManagerStatus>(queryKeys.status, (current) =>
        current
          ? {
              ...current,
              detection: view.detection,
              engine: view.detection.engine,
              settings: { ...current.settings, ...view.settings },
            }
          : current,
      );
      setForm(view.settings);
      setDirty(false);
      invalidate();
      void queryClient.invalidateQueries({ queryKey: queryKeys.diagnostics, exact: true });
      toast({
        title: "Settings saved",
        description: view.detection.found
          ? `${view.detection.engine || "engine"} ${view.detection.version || ""} is ready.`.trim()
          : view.detection.engine && view.detection.engine !== "auto"
            ? `Saved, but ${view.detection.engine} is still not detected. Check the binary path.`
            : "Saved, but no version manager was detected. Check the binary path.",
        variant: view.detection.found ? "success" : "warning",
      });
    },
    onError: (error) => {
      applyTheme(savedTheme.current, { persist: false });
      setForm((current) => ({ ...current, theme: savedTheme.current }));
      toast({ title: "Could not save settings", description: errorMessage(error), variant: "error" });
    },
  });

  const reset = useMutation({
    mutationFn: () => SettingsService.Reset(),
    onSuccess: (view) => {
      const theme = normalizeTheme(view.settings.theme);
      applyTheme(theme);
      queryClient.setQueryData(queryKeys.settings, view);
      queryClient.setQueryData<ManagerStatus>(queryKeys.status, (current) =>
        current
          ? {
              ...current,
              detection: view.detection,
              engine: view.detection.engine,
              settings: { ...current.settings, ...view.settings },
            }
          : current,
      );
      setForm(view.settings);
      setDirty(false);
      setResetOpen(false);
      invalidate();
      toast({
        title: "Overrides cleared",
        description: "Engine discovery is back to automatic detection.",
        variant: "info",
      });
    },
    onError: (error) => {
      toast({ title: "Could not reset settings", description: errorMessage(error), variant: "error" });
    },
  });

  const update = <K extends keyof SettingsModel>(key: K, value: SettingsModel[K]) => {
    setForm((current) => ({ ...current, [key]: value }));
    setDirty(true);
  };

  const pick = async (
    picker: () => Promise<string>,
    key: "binaryPath" | "dataDir" | "defaultProjectDir",
  ) => {
    try {
      const path = await picker();
      if (path) update(key, path);
    } catch (error) {
      toast({ title: "Folder picker failed", description: errorMessage(error), variant: "error" });
    }
  };

  const redetect = async () => {
    try {
      const next = await ManagerService.Diagnostics();
      queryClient.setQueryData(queryKeys.diagnostics, next);
      setDiagnosticsEnabled(true);
      void queryClient.invalidateQueries({ queryKey: queryKeys.status });
      void queryClient.invalidateQueries({ queryKey: queryKeys.settings });
      toast({
        title: next.detection.found ? `${next.detection.engine || "engine"} detected` : "engine not found",
        description: next.detection.found
          ? `Using ${next.detection.binaryPath}`
          : next.detection.error || "Set the binary path below.",
        variant: next.detection.found ? "success" : "warning",
      });
    } catch (error) {
      toast({ title: "Re-detection failed", description: errorMessage(error), variant: "error" });
    }
  };

  const openPath = async (action: () => Promise<void>, label: string) => {
    try {
      await action();
    } catch (error) {
      toast({ title: `Could not open ${label}`, description: errorMessage(error), variant: "error" });
    }
  };

  const canSave = dirty && form.theme !== "";
  const detection = settings.data?.detection ?? status.data?.detection;
  const activeEngine = form.engine === "auto" ? detection?.engine || "auto" : form.engine;
  const engineLabel = activeEngine === "auto" ? "auto-detect" : activeEngine;
  const isMise = activeEngine === "mise";
  const isWSLRunner = form.runnerMode === "wsl";
  const binaryLabel = isWSLRunner
    ? isMise
      ? "mise binary in WSL"
      : "asdf binary in WSL"
    : isMise
      ? "mise binary"
      : form.engine === "asdf"
        ? "asdf binary"
        : "Version manager binary";
  const dataDirLabel = isMise ? "MISE_DATA_DIR" : form.engine === "asdf" ? "ASDF_DATA_DIR" : "Data directory";
  const binaryPlaceholder = isWSLRunner
    ? "auto-detect inside WSL (e.g. /home/you/.asdf/bin/asdf)"
    : isMise
      ? "auto-detect (e.g. ~/.local/bin/mise)"
      : "auto-detect (e.g. ~/.asdf/bin/asdf)";
  const dataDirPlaceholder = isWSLRunner
    ? isMise
      ? "auto-detect inside WSL (e.g. /home/you/.local/share/mise)"
      : "auto-detect inside WSL (e.g. /home/you/.asdf)"
    : isMise
      ? "auto-detect (e.g. ~/.local/share/mise)"
      : "auto-detect (e.g. ~/.asdf)";
  const docsURL = isMise ? "https://mise.jdx.dev/" : "https://asdf-vm.com/guide/getting-started.html";
  const infoLabel = `${activeEngine === "auto" ? "Engine" : activeEngine} info`;

  const updater = updateStatus.data;
  const updateState = updater?.state ?? "idle";
  const updateBusy = checkUpdate.isPending || updateState === "checking";

  const installNow = () => {
    installUpdate.mutate(undefined, {
      onError: (error) => {
        toast({ title: "Update failed", description: errorMessage(error), variant: "error" });
      },
    });
  };

  const checkNow = () => {
    checkUpdate.mutate(undefined, {
      onSuccess: (next) => {
        if (next.state === "up-to-date") {
          toast({
            title: "You are up to date",
            description: `Version ${next.currentVersion} is the latest release.`,
            variant: "success",
          });
        } else if (next.state === "available") {
          toast({
            title: `Version ${next.latestVersion} is available`,
            description: next.installable
              ? "Install it in place here, or read the release notes first."
              : "Download it from GitHub Releases. This build cannot replace itself.",
            variant: "info",
          });
        } else {
          toast({
            title: "Update check failed",
            description: next.error || "GitHub did not return a release.",
            variant: "error",
          });
        }
      },
      onError: (error) => {
        toast({ title: "Update check failed", description: errorMessage(error), variant: "error" });
      },
    });
  };

  return (
    <div className="space-y-5">
      <PageHeader
        eyebrow="Preferences"
        title="Settings"
        description="Choose asdf or mise, point the app at a specific installation, choose a default workspace, and inspect the environment the GUI will drive."
        actions={
          <>
            <Button variant="ghost" size="sm" onClick={() => setResetOpen(true)} disabled={reset.isPending}>
              <RotateCcw className="size-3.5" />
              Reset overrides
            </Button>
            <Button
              size="sm"
              disabled={!canSave}
              loading={save.isPending}
              onClick={() => save.mutate(form)}
            >
              <Save className="size-3.5" />
              Save changes
            </Button>
          </>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,1fr)]">
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <div>
                <CardTitle>Engine</CardTitle>
                <p className="mt-1 text-[11px] leading-relaxed text-muted">
                  Auto prefers asdf when both are installed, then falls back to mise. Leave the paths empty to discover
                  the selected engine from <span className="font-mono">PATH</span> and standard locations.
                </p>
              </div>
              <Button variant="ghost" size="sm" className="h-7 text-[11px]" onClick={redetect}>
                <RefreshCw className="size-3" />
                Re-detect
              </Button>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>Version manager</Label>
                <div className="grid gap-2 sm:grid-cols-3">
                  {ENGINES.map((engine) => {
                    const active = form.engine === engine.value;
                    return (
                      <button
                        key={engine.value}
                        onClick={() => {
                          if (engine.value === form.engine) return;
                          // Overrides are engine-specific: an asdf binary and
                          // data directory must never be handed to mise.
                          setForm((current) => ({
                            ...current,
                            engine: engine.value,
                            binaryPath: "",
                            dataDir: "",
                          }));
                          setDirty(true);
                        }}
                        aria-pressed={active}
                        className={cn(
                          "rounded-lg border px-3 py-2.5 text-left transition",
                          active
                            ? "border-accent/40 bg-accent/[0.07]"
                            : "border-line bg-canvas/40 hover:border-line-strong",
                        )}
                      >
                        <div className="flex items-center justify-between">
                          <span className={cn("text-[12px] font-medium", active ? "text-ink" : "text-muted")}>
                            {engine.label}
                          </span>
                          {active ? <CheckCircle2 className="size-3.5 text-accent" /> : null}
                        </div>
                        <div className="mt-0.5 text-[10px] leading-relaxed text-faint">{engine.hint}</div>
                      </button>
                    );
                  })}
                </div>
                <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                  Switching engines clears the binary and data directory overrides so an asdf path is never used for
                  mise, or the reverse.
                </p>
              </div>

              {status.data?.os === "windows" ? (
                <div>
                  <Label>Runner</Label>
                  <div className="grid gap-2 sm:grid-cols-3">
                    {RUNNERS.map((runner) => {
                      const active = form.runnerMode === runner.value;
                      return (
                        <button
                          key={runner.value}
                          onClick={() => {
                            if (runner.value === form.runnerMode) return;
                            setForm((current) => ({
                              ...current,
                              runnerMode: runner.value,
                              binaryPath: "",
                              dataDir: "",
                            }));
                            setDirty(true);
                          }}
                          aria-pressed={active}
                          className={cn(
                            "rounded-lg border px-3 py-2.5 text-left transition",
                            active
                              ? "border-accent/40 bg-accent/[0.07]"
                              : "border-line bg-canvas/40 hover:border-line-strong",
                          )}
                        >
                          <div className="flex items-center justify-between">
                            <span className={cn("text-[12px] font-medium", active ? "text-ink" : "text-muted")}>
                              {runner.label}
                            </span>
                            {active ? <CheckCircle2 className="size-3.5 text-accent" /> : null}
                          </div>
                          <div className="mt-0.5 text-[10px] leading-relaxed text-faint">{runner.hint}</div>
                        </button>
                      );
                    })}
                  </div>
                  <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                    Auto prefers a native install, then falls back to WSL2. WSL2 runs the Linux CLI inside the selected
                    distribution and opens project files through <span className="font-mono">\\wsl.localhost</span>.
                  </p>
                  {form.runnerMode === "wsl" ? (
                    <div className="mt-3">
                      <Label>WSL distribution</Label>
                      <Input
                        value={form.wslDistro}
                        onChange={(event) => update("wslDistro", event.target.value)}
                        placeholder={
                          (wslDistros.data ?? []).length > 0
                            ? `auto (default: ${(wslDistros.data ?? [])[0]})`
                            : "auto (default distribution)"
                        }
                        list="wsl-distribution-options"
                        className="font-mono text-[12px]"
                      />
                      <datalist id="wsl-distribution-options">
                        {(wslDistros.data ?? []).map((distro) => (
                          <option key={distro} value={distro} />
                        ))}
                      </datalist>
                      <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                        Leave empty to use the default distribution. Install asdf or mise inside WSL and keep its data
                        directory on the Linux filesystem.
                      </p>
                      {wslDistros.isError ? (
                        <p className="mt-1 text-[10px] text-warn">
                          Could not list distributions: {errorMessage(wslDistros.error)}
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              ) : null}

              <div>
                <Label>{binaryLabel}</Label>
                <div className="flex gap-2">
                  <Input
                    value={form.binaryPath}
                    onChange={(event) => update("binaryPath", event.target.value)}
                    placeholder={binaryPlaceholder}
                    className="font-mono text-[12px]"
                  />
                  <Button variant="secondary" onClick={() => pick(SettingsService.PickBinary, "binaryPath")}>
                    <FolderOpen className="size-3.5" />
                    Browse
                  </Button>
                </div>
                <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                  Must be an executable {isMise ? "mise" : "version manager"} CLI. The app never reimplements{" "}
                  {isMise ? "mise" : "asdf"} commands.
                </p>
              </div>

              <div>
                <Label>{dataDirLabel}</Label>
                <div className="flex gap-2">
                  <Input
                    value={form.dataDir}
                    onChange={(event) => update("dataDir", event.target.value)}
                    placeholder={dataDirPlaceholder}
                    className="font-mono text-[12px]"
                  />
                  <Button variant="secondary" onClick={() => pick(SettingsService.PickDataDir, "dataDir")}>
                    <FolderOpen className="size-3.5" />
                    Browse
                  </Button>
                </div>
                <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                  Where plugins and installed versions live. Useful when {engineLabel} is installed through Homebrew,
                  Nix, or a custom prefix.
                </p>
              </div>

              <div>
                <Label>Default project directory</Label>
                <div className="flex gap-2">
                  <Input
                    value={form.defaultProjectDir}
                    onChange={(event) => update("defaultProjectDir", event.target.value)}
                    placeholder="optional (e.g. ~/code/my-app)"
                    className="font-mono text-[12px]"
                  />
                  <Button variant="secondary" onClick={() => pick(SettingsService.PickProjectDir, "defaultProjectDir")}>
                    <FolderOpen className="size-3.5" />
                    Browse
                  </Button>
                </div>
                <p className="mt-1.5 text-[11px] leading-relaxed text-faint">
                  The Project screen and dashboard open this directory by default.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div>
                <CardTitle>Appearance</CardTitle>
                <p className="mt-1 text-[11px] leading-relaxed text-muted">
                  The theme applies to the whole desktop app and is previewed as you choose. Save changes to keep it
                  across restarts.
                </p>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid gap-2 sm:grid-cols-3">
                {THEMES.map((theme) => {
                  const Icon = theme.icon;
                  const active = form.theme === theme.value;
                  return (
                    <button
                      key={theme.value}
                      onClick={() => {
                        update("theme", theme.value);
                        applyTheme(theme.value, { persist: false });
                      }}
                      aria-pressed={active}
                      className={cn(
                        "rounded-lg border px-3 py-3 text-left transition",
                        active
                          ? "border-accent/40 bg-accent/[0.07]"
                          : "border-line bg-canvas/40 hover:border-line-strong",
                      )}
                    >
                      <div className="flex items-center justify-between">
                        <Icon className={cn("size-4", active ? "text-accent" : "text-muted")} />
                        {active ? <CheckCircle2 className="size-3.5 text-accent" /> : null}
                      </div>
                      <div className={cn("mt-2 text-[12px] font-medium", active ? "text-ink" : "text-muted")}>
                        {theme.label}
                      </div>
                      <div className="mt-0.5 text-[10px] leading-relaxed text-faint">{theme.hint}</div>
                    </button>
                  );
                })}
              </div>
              <p className="mt-2 text-[10px] leading-relaxed text-faint">
                Click a theme to preview it now. Save changes to keep it after restarting the app.
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div>
                <CardTitle>Diagnostics</CardTitle>
                <p className="mt-1 text-[11px] leading-relaxed text-muted">
                  Verifies bash, git, the data directory, and shell integration for {engineLabel}. Runs live shell
                  probes.
                </p>
              </div>
              <Button
                variant={diagnosticsEnabled ? "secondary" : "primary"}
                size="sm"
                className="h-7"
                loading={diagnostics.isFetching}
                onClick={() => {
                  setDiagnosticsEnabled(true);
                  if (diagnosticsEnabled) void diagnostics.refetch();
                }}
              >
                <FileCog className="size-3.5" />
                {diagnosticsEnabled ? "Re-run" : "Run diagnostics"}
              </Button>
            </CardHeader>
            <CardContent>
              {!diagnosticsEnabled ? (
                <div className="rounded-lg border border-dashed border-line px-4 py-6 text-center">
                  <Settings2 className="mx-auto size-5 text-faint" />
                  <p className="mt-2 text-[11px] leading-relaxed text-faint">
                    Run diagnostics to see exactly what the app can see. No data leaves this machine.
                  </p>
                </div>
              ) : diagnostics.isLoading ? (
                <div className="space-y-2">
                  <div className="skeleton h-8 rounded-lg" />
                  <div className="skeleton h-8 rounded-lg" />
                  <div className="skeleton h-8 rounded-lg" />
                </div>
              ) : diagnostics.isError ? (
                <p className="text-xs text-danger">{errorMessage(diagnostics.error)}</p>
              ) : (
                <div className="space-y-3">
                  <div className="space-y-1.5">
                    {(diagnostics.data?.checks ?? []).map((check) => (
                      <CheckRow key={check.name} name={check.name} status={check.status} detail={check.detail} />
                    ))}
                  </div>

                  {diagnostics.data?.engineInfo ? (
                    <div>
                      <Label className="mb-1.5">{infoLabel}</Label>
                      <pre className="max-h-40 overflow-auto rounded-lg border border-line bg-canvas/60 p-3 font-mono text-[10.5px] leading-relaxed text-muted">
                        {diagnostics.data.engineInfo}
                      </pre>
                    </div>
                  ) : null}

                  <div className="grid gap-x-4 gap-y-1.5 rounded-lg border border-line-soft bg-canvas/30 p-3 text-[11px] sm:grid-cols-2">
                    <DiagFact label="Platform" value={`${diagnostics.data?.os ?? "—"} / ${diagnostics.data?.arch ?? "—"}`} />
                    <DiagFact label="Environment" value={diagnostics.data?.environment ?? "—"} />
                    <DiagFact label="Shell" value={diagnostics.data?.shell || "unknown"} mono />
                    <DiagFact label="bash" value={firstLine(diagnostics.data?.bashVersion) || "missing"} />
                    <DiagFact label="git" value={firstLine(diagnostics.data?.gitVersion) || "missing"} />
                    <DiagFact label="Generated" value={diagnostics.data?.generatedAt ?? "—"} />
                  </div>

                  <details className="rounded-lg border border-line-soft bg-canvas/30 px-3 py-2">
                    <summary className="cursor-pointer text-[11px] text-muted">PATH environment</summary>
                    <pre className="mt-2 whitespace-pre-wrap break-all font-mono text-[10px] leading-relaxed text-faint">
                      {diagnostics.data?.pathEnv || "—"}
                    </pre>
                  </details>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Detected engine</CardTitle>
              <Badge variant={detection?.found ? "success" : "danger"}>
                {detection?.found ? "connected" : "not found"}
              </Badge>
            </CardHeader>
            <CardContent className="space-y-3 pt-3">
              <PathRow label="Engine" value={detection?.engine || activeEngine || "—"} />
              <PathRow label="Binary" value={detection?.binaryPath || "—"} mono />
              <PathRow label="Version" value={detection?.version || "—"} mono />
              <PathRow label="Data dir" value={detection?.dataDir || "—"} mono />
              <PathRow label="Source" value={detection?.source || "—"} />
              <PathRow label="Checked" value={detection?.checkedAt ? new Date(detection.checkedAt).toLocaleString() : "—"} />
              {detection?.error ? (
                <div className="rounded-lg border border-danger/20 bg-danger/5 px-3 py-2 font-mono text-[10.5px] leading-relaxed text-danger">
                  {detection.error}
                </div>
              ) : null}
              <div className="flex flex-wrap gap-2 border-t border-line-soft pt-3">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => openPath(ManagerService.OpenConfigDir, "config folder")}
                >
                  <FolderOpen className="size-3.5" />
                  Config folder
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => openPath(ManagerService.OpenLogsDir, "logs folder")}
                >
                  <Terminal className="size-3.5" />
                  Logs
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Local data</CardTitle>
              <Database className="size-3.5 text-faint" />
            </CardHeader>
            <CardContent className="space-y-3 pt-3">
              <PathRow label="Settings" value={settings.data?.settingsPath || "—"} mono />
              <PathRow label="Logs" value={settings.data?.logsDir || "—"} mono />
              <PathRow label="Config" value={settings.data?.configDir || "—"} mono />
              <p className="text-[10px] leading-relaxed text-faint">
                Settings are stored as JSON on this machine. Project files stay in the projects you choose.
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div>
                <CardTitle>Updates</CardTitle>
                <p className="mt-1 text-[11px] leading-relaxed text-muted">
                  Checks GitHub Releases at launch and on demand. The AppImage verifies a SHA-256 checksum and updates in
                  place; every other build links to the release download.
                </p>
              </div>
              <Badge variant={updateBadgeVariant(updateState)}>{updateBadgeLabel(updateState)}</Badge>
            </CardHeader>
            <CardContent className="space-y-3 pt-3">
              <div className="grid grid-cols-2 gap-3 rounded-lg border border-line-soft bg-canvas/30 px-3 py-2.5">
                <VersionCell label="Current" value={updater?.currentVersion || status.data?.appVersion || "—"} />
                <VersionCell label="Latest" value={updater?.latestVersion || "—"} />
              </div>

              {updateState === "error" && updater?.error ? (
                <div className="rounded-lg border border-danger/20 bg-danger/5 px-3 py-2 text-[10.5px] leading-relaxed text-danger">
                  {updater.error}
                </div>
              ) : null}

              {updateState === "up-to-date" ? (
                <div className="flex items-start gap-2 rounded-lg border border-accent/20 bg-accent/5 px-3 py-2 text-[10.5px] leading-relaxed text-accent">
                  <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
                  You are running the latest release.
                </div>
              ) : null}

              {updateState === "available" ? (
                <div className="space-y-2">
                  <div className="flex items-start gap-2 rounded-lg border border-info/20 bg-info/5 px-3 py-2">
                    <Sparkles className="mt-0.5 size-3.5 shrink-0 text-info" />
                    <div className="min-w-0 flex-1">
                      <p className="text-[11px] font-medium text-ink">
                        {updater?.releaseName || `Version ${updater?.latestVersion ?? "unknown"}`}
                      </p>
                      <p className="mt-0.5 text-[10px] leading-relaxed text-faint">
                        {updater?.publishedAt
                          ? `Published ${new Date(updater.publishedAt).toLocaleDateString()}`
                          : "Published on GitHub Releases"}
                        {updater?.assetName ? ` · ${updater.assetName}` : ""}
                        {updater?.assetSize ? ` · ${formatBytes(updater.assetSize)}` : ""}
                      </p>
                    </div>
                  </div>
                  {updater?.releaseNotes ? (
                    <details className="rounded-lg border border-line-soft bg-canvas/30 px-3 py-2">
                      <summary className="cursor-pointer text-[11px] text-muted">Release notes</summary>
                      <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap break-words font-sans text-[10.5px] leading-relaxed text-faint">
                        {updater.releaseNotes}
                      </pre>
                    </details>
                  ) : null}
                </div>
              ) : null}

              <div className="flex flex-wrap gap-2">
                <Button variant="secondary" size="sm" loading={updateBusy} onClick={checkNow}>
                  <RefreshCw className={cn("size-3.5", updateBusy && "animate-spin")} />
                  Check now
                </Button>
                {updateState === "available" && updater?.installable ? (
                  <Button size="sm" loading={installUpdate.isPending} onClick={installNow}>
                    <Download className="size-3.5" />
                    Install update
                  </Button>
                ) : null}
                {updateState === "available" && !updater?.installable ? (
                  <Button size="sm" onClick={() => void ManagerService.OpenURL(updater?.releaseURL || RELEASES_URL)}>
                    <ExternalLink className="size-3.5" />
                    Download release
                  </Button>
                ) : null}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => void ManagerService.OpenURL(updater?.releaseURL || RELEASES_URL)}
                >
                  View release
                </Button>
              </div>

              <div className="flex items-start gap-2 rounded-lg border border-line-soft bg-canvas/30 px-3 py-2.5 text-[10.5px] leading-relaxed text-muted">
                <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-accent" />
                {updater?.canSelfUpdate
                  ? "Downloads are verified against the release's SHA-256 checksum before the AppImage is replaced. Settings and projects are kept."
                  : "This build cannot replace itself. Downloading opens GitHub Releases; settings and projects stay untouched."}
              </div>

              <div className="flex items-start justify-between gap-3 border-t border-line-soft pt-3">
                <div className="min-w-0">
                  <div className="text-[11px] font-medium text-ink">Check at launch</div>
                  <p className="mt-0.5 text-[10px] leading-relaxed text-faint">
                    Ask GitHub for the latest release when the app opens. No account, telemetry, or personal data is
                    involved. Save changes to apply.
                  </p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={form.checkForUpdates}
                  aria-label="Check for updates at launch"
                  onClick={() => update("checkForUpdates", !form.checkForUpdates)}
                  className={cn(
                    "relative mt-0.5 h-5 w-9 shrink-0 rounded-full border transition",
                    form.checkForUpdates ? "border-accent/40 bg-accent" : "border-line bg-elevated",
                  )}
                >
                  <span
                    className={cn(
                      "absolute top-1/2 size-3.5 -translate-y-1/2 rounded-full bg-white shadow transition-[left]",
                      form.checkForUpdates ? "left-[18px]" : "left-[2px]",
                    )}
                  />
                </button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>About</CardTitle>
              <Info className="size-3.5 text-faint" />
            </CardHeader>
            <CardContent className="space-y-3 pt-3">
              <div className="flex items-center justify-between">
                <span className="text-[11px] text-faint">Application</span>
                <span className="font-mono text-[11px] text-muted">
                  asdf mise GUI {status.data?.appVersion || updater?.currentVersion || "—"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[11px] text-faint">Engine</span>
                <span className="font-mono text-[11px] text-muted">
                  {(detection?.engine || activeEngine || "asdf")} {detection?.version || "—"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[11px] text-faint">Platform</span>
                <span className="text-[11px] text-muted">
                  {status.data?.environment ?? "—"}
                  {status.data?.wsl ? " (WSL2)" : ""}
                </span>
              </div>
              {status.data?.os === "windows" ? (
                <div className="flex items-start gap-2 rounded-lg border border-warn/20 bg-warn/5 px-3 py-2 text-[10.5px] leading-relaxed text-warn">
                  <AlertTriangle className="mt-0.5 size-3 shrink-0" />
                  Native Windows is not supported. Run the Linux build inside WSL2.
                </div>
              ) : null}
              <div className="flex flex-wrap gap-2 border-t border-line-soft pt-3">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => void ManagerService.OpenURL(docsURL)}
                >
                  {isMise ? "mise docs" : "asdf docs"}
                  <ExternalLink className="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => void ManagerService.OpenURL(REPO_URL)}
                >
                  GitHub
                  <ExternalLink className="size-3.5" />
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      <ConfirmDialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        title="Reset all overrides?"
        description="Clears the custom binary path, data directory, and default project. Theme and recent projects are kept. Discovery returns to automatic detection."
        confirmLabel="Reset settings"
        destructive
        loading={reset.isPending}
        onConfirm={() => reset.mutate()}
      />
    </div>
  );
}

function CheckRow({ name, status, detail }: { name: string; status: string; detail: string }) {
  const icon =
    status === "ok" ? (
      <CheckCircle2 className="size-3.5 shrink-0 text-accent" />
    ) : status === "warn" ? (
      <AlertTriangle className="size-3.5 shrink-0 text-warn" />
    ) : (
      <XCircle className="size-3.5 shrink-0 text-danger" />
    );

  return (
    <div className="flex items-start gap-2.5 rounded-lg border border-line-soft bg-canvas/30 px-3 py-2">
      <span className="mt-0.5">{icon}</span>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-[11.5px] font-medium text-ink">{name}</span>
          <span
            className={cn(
              "rounded-full px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide",
              status === "ok" && "bg-accent/10 text-accent",
              status === "warn" && "bg-warn/10 text-warn",
              status !== "ok" && status !== "warn" && "bg-danger/10 text-danger",
            )}
          >
            {statusLabel(status)}
          </span>
        </div>
        <p className="mt-0.5 break-words font-mono text-[10px] leading-relaxed text-faint" title={detail}>
          {detail}
        </p>
      </div>
    </div>
  );
}

type BadgeVariant = "default" | "accent" | "success" | "warning" | "danger" | "info" | "outline" | "solid";

function updateBadgeVariant(state: string): BadgeVariant {
  switch (state) {
    case "available":
      return "info";
    case "up-to-date":
      return "success";
    case "downloading":
    case "checking":
      return "info";
    case "ready":
      return "accent";
    case "error":
      return "danger";
    default:
      return "default";
  }
}

function updateBadgeLabel(state: string): string {
  switch (state) {
    case "available":
      return "Update available";
    case "up-to-date":
      return "Up to date";
    case "checking":
      return "Checking";
    case "downloading":
      return "Downloading";
    case "ready":
      return "Ready to restart";
    case "error":
      return "Check failed";
    default:
      return "Not checked";
  }
}

function VersionCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[10px] uppercase tracking-[0.08em] text-faint">{label}</div>
      <div className="mt-0.5 truncate font-mono text-[12px] text-muted" title={value}>
        {value}
      </div>
    </div>
  );
}

function PathRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <div className="text-[10px] uppercase tracking-[0.08em] text-faint">{label}</div>
      <div
        className={cn("mt-0.5 truncate text-[11px] text-muted", mono && "font-mono text-[10.5px]")}
        title={value}
      >
        {value.startsWith("/") ? shortenPath(value, 46) : value}
      </div>
    </div>
  );
}

function DiagFact({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex min-w-0 items-baseline justify-between gap-2">
      <span className="shrink-0 text-[10px] text-faint">{label}</span>
      <span className={cn("min-w-0 truncate text-right text-[10.5px] text-muted", mono && "font-mono")} title={value}>
        {value}
      </span>
    </div>
  );
}

function firstLine(value?: string): string {
  if (!value) return "";
  return value.split("\n")[0].trim();
}
