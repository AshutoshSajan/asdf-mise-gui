import { useEffect, useState } from "react";
import { Download, ExternalLink, RefreshCw, ShieldCheck, Sparkles, X } from "lucide-react";
import { Events } from "@wailsio/runtime";
import { Button } from "./ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";
import { useInstallUpdate, useUpdateStatus } from "../lib/query";
import { errorMessage, formatBytes } from "../lib/format";
import { useRoute } from "../lib/route";
import { ManagerService } from "../lib/bindings";
import { cn } from "../lib/utils";

const DownloadStarted = "wails:updater:download-started";
const DownloadProgress = "wails:updater:download-progress";
const Verifying = "wails:updater:verifying";
const Installing = "wails:updater:installing";
const UpdateReady = "wails:updater:update-ready";
const UpdateError = "wails:updater:error";

type DownloadProgressPayload = {
  written: number;
  total: number;
  rate: number;
};

type UpdateErrorPayload = {
  stage?: string;
  message?: string;
};

type Phase = "downloading" | "verifying" | "installing" | "ready" | "error";

export function UpdateCenter() {
  const route = useRoute();
  const update = useUpdateStatus();
  const install = useInstallUpdate();
  const [dismissed, setDismissed] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [phase, setPhase] = useState<Phase>("downloading");
  const [progress, setProgress] = useState<DownloadProgressPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const offStarted = Events.On(DownloadStarted, () => {
      setError(null);
      setProgress(null);
      setPhase("downloading");
      setDialogOpen(true);
    });
    const offProgress = Events.On(DownloadProgress, (event) => {
      setProgress(event.data as DownloadProgressPayload);
    });
    const offVerifying = Events.On(Verifying, () => setPhase("verifying"));
    const offInstalling = Events.On(Installing, () => setPhase("installing"));
    const offReady = Events.On(UpdateReady, () => setPhase("ready"));
    const offError = Events.On(UpdateError, (event) => {
      const payload = event.data as UpdateErrorPayload | null;
      // A background check failure is reported in Settings. Only download,
      // verify, and install failures interrupt the user with a dialog.
      if (!payload?.stage || payload.stage === "check") return;
      setError(payload.message || "The update could not be installed.");
      setPhase("error");
      setDialogOpen(true);
    });

    return () => {
      offStarted();
      offProgress();
      offVerifying();
      offInstalling();
      offReady();
      offError();
    };
  }, []);

  const status = update.data;
  const latest = status?.latestVersion ?? "";
  // Settings renders the full update card, so the global banner would be a
  // duplicate on that screen. The install dialog is independent of the banner.
  const bannerVisible =
    route.path !== "/settings" &&
    Boolean(latest) &&
    dismissed !== latest &&
    (status?.state === "available" || status?.state === "downloading" || status?.state === "ready");

  const startInstall = () => {
    setError(null);
    setProgress(null);
    setPhase("downloading");
    setDialogOpen(true);
    install.mutate(undefined, {
      onError: (installError) => {
        setError(errorMessage(installError));
        setPhase("error");
        setDialogOpen(true);
      },
      onSuccess: () => setPhase("ready"),
    });
  };

  const closeDialog = (open: boolean) => {
    if (!open && (phase === "downloading" || phase === "verifying" || phase === "installing")) {
      return;
    }
    setDialogOpen(open);
  };

  const total = progress?.total || status?.assetSize || 0;
  const written = progress?.written || 0;
  const percent = total > 0 ? Math.min(100, Math.round((written / total) * 100)) : null;

  return (
    <>
      {bannerVisible ? (
        <div className="no-drag flex shrink-0 items-center gap-3 border-b border-info/20 bg-info/[0.06] px-6 py-2.5">
          <span className="grid size-7 shrink-0 place-items-center rounded-full border border-info/25 bg-info/10">
            <Sparkles className="size-3.5 text-info" />
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-[12px] font-medium text-ink">
              Version {latest} is available
            </p>
            <p className="truncate text-[10.5px] text-faint">
              You are running {status?.currentVersion ?? "an older build"}.
              {status?.installable
                ? " The AppImage can install it in place and restart."
                : " Download the latest release from GitHub."}
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {status?.installable ? (
              <Button size="sm" loading={install.isPending} onClick={startInstall}>
                <Download className="size-3.5" />
                Update now
              </Button>
            ) : (
              <Button size="sm" onClick={() => void ManagerService.OpenURL(status?.releaseURL ?? "")}>
                <ExternalLink className="size-3.5" />
                Download
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void ManagerService.OpenURL(status?.releaseURL ?? "")}
            >
              Release notes
            </Button>
            <button
              className="rounded p-1 text-faint transition hover:bg-elevated hover:text-ink"
              onClick={() => setDismissed(latest)}
              aria-label="Dismiss update notification"
            >
              <X className="size-4" />
            </button>
          </div>
        </div>
      ) : null}

      <Dialog open={dialogOpen} onOpenChange={closeDialog}>
        <DialogContent showClose={phase === "error"}>
          <DialogHeader>
            <DialogTitle>
              {phase === "error"
                ? "Update failed"
                : phase === "ready"
                  ? "Update ready"
                  : "Installing update"}
            </DialogTitle>
            <DialogDescription>
              {phase === "error"
                ? "Your current version is unchanged. You can retry from Settings or download the release manually."
                : phase === "ready"
                  ? `Version ${latest || "the new release"} is verified. The app will restart and finish installing it now.`
                  : `Downloading and verifying version ${latest || "the new release"} before replacing the AppImage.`}
            </DialogDescription>
          </DialogHeader>

          <DialogBody>
            {phase === "error" ? (
              <div className="rounded-lg border border-danger/20 bg-danger/5 px-3 py-2.5 text-[11px] leading-relaxed text-danger">
                {error ?? "The update could not be installed."}
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex items-center justify-between gap-3 text-[11px]">
                  <span className="text-muted">{phaseLabel(phase)}</span>
                  {percent !== null ? <span className="font-mono text-faint">{percent}%</span> : null}
                </div>
                <div className="h-1.5 overflow-hidden rounded-full bg-elevated">
                  <div
                    className={cn(
                      "h-full rounded-full bg-accent transition-[width] duration-200",
                      percent === null && "w-1/3 animate-pulse",
                    )}
                    style={percent !== null ? { width: `${percent}%` } : undefined}
                  />
                </div>
                <div className="flex items-center justify-between gap-3 text-[10px] text-faint">
                  <span>
                    {progress
                      ? `${formatBytes(written)} of ${total > 0 ? formatBytes(total) : "unknown"}`
                      : "Connecting to GitHub…"}
                  </span>
                  <span>{progress && progress.rate > 0 ? `${formatBytes(progress.rate)}/s` : ""}</span>
                </div>
                <div className="flex items-start gap-2 rounded-lg border border-line-soft bg-canvas/30 px-3 py-2.5 text-[10.5px] leading-relaxed text-muted">
                  <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-accent" />
                  The download is checked against the release&apos;s SHA-256 checksum and Ed25519 signature before the
                  AppImage is replaced. The app restarts only after verification succeeds.
                </div>
              </div>
            )}
          </DialogBody>

          <DialogFooter>
            {phase === "error" ? (
              <>
                <Button variant="ghost" onClick={() => setDialogOpen(false)}>
                  Close
                </Button>
                <Button variant="secondary" onClick={startInstall} loading={install.isPending}>
                  <RefreshCw className="size-3.5" />
                  Try again
                </Button>
              </>
            ) : (
              <span className="text-[10px] text-faint">Keep the app open until it restarts.</span>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function phaseLabel(phase: Phase): string {
  switch (phase) {
    case "verifying":
      return "Verifying SHA-256 checksum…";
    case "installing":
      return "Installing update…";
    case "ready":
      return "Restarting…";
    default:
      return "Downloading update…";
  }
}
