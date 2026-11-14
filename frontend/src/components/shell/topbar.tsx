import { RefreshCw, Terminal } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useOperations, useStatus } from "../../lib/query";
import { navigate } from "../../lib/route";
import { cn } from "../../lib/utils";
import { Button } from "../ui/button";
import { Hint } from "../ui/tooltip";
import { isActiveOperation } from "../operation-status";

export function Topbar({ title }: { title: string }) {
  const status = useStatus();
  const operations = useOperations();
  const queryClient = useQueryClient();
  const active = (operations.data ?? []).filter(isActiveOperation);
  const running = active.find((operation) => operation.status === "running") ?? active[0];

  const refresh = () => {
    void queryClient.invalidateQueries();
  };

  return (
    <header className="drag-region flex h-[64px] shrink-0 items-center justify-between gap-4 border-b border-line px-6">
      <div className="flex min-w-0 items-center gap-3">
        <span className="text-[12px] font-medium text-muted">{title}</span>
        {status.data?.detection.found ? (
          <>
            <span className="text-faint">/</span>
            <span className="font-mono text-[11px] text-faint">{status.data.detection.version}</span>
          </>
        ) : null}
      </div>

      <div className="no-drag flex items-center gap-2">
        {running ? (
          <button
            onClick={() => navigate("/operations", { op: running.id })}
            className="flex max-w-[280px] items-center gap-2 rounded-full border border-info/25 bg-info/10 px-3 py-1.5 text-[11px] text-info transition hover:bg-info/15"
          >
            <RefreshCw className="size-3 animate-spin" />
            <span className="truncate">{running.title}</span>
            {active.length > 1 ? (
              <span className="rounded-full bg-info/20 px-1.5 text-[10px] font-semibold">+{active.length - 1}</span>
            ) : null}
          </button>
        ) : null}
        <Hint label="Refresh all data">
          <Button variant="ghost" size="icon" onClick={refresh} aria-label="Refresh">
            <RefreshCw className={cn("size-4", status.isFetching && "animate-spin")} />
          </Button>
        </Hint>
        <Hint label="Open operations">
          <Button variant="ghost" size="icon" onClick={() => navigate("/operations")} aria-label="Operations">
            <Terminal className="size-4" />
          </Button>
        </Hint>
      </div>
    </header>
  );
}
