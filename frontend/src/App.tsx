import { useEffect, useRef } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import {
  OperationFinished,
  OperationQueued,
  OperationStatus,
  OperationUpdated,
  StateChanged,
} from "./lib/bindings";
import { useInvalidateAppData, useStatus } from "./lib/query";
import { useRoute } from "./lib/route";
import { applyTheme, normalizeTheme, readStoredTheme } from "./lib/theme";
import { Sidebar } from "./components/shell/sidebar";
import { Topbar } from "./components/shell/topbar";
import { StatusBar } from "./components/shell/status-bar";
import { TooltipProvider } from "./components/ui/tooltip";
import { ToastProvider, useToast } from "./components/ui/toast";
import { Dashboard } from "./screens/dashboard";
import { Tools } from "./screens/tools";
import { Plugins } from "./screens/plugins";
import { Project } from "./screens/project";
import { Operations } from "./screens/operations";
import { Settings } from "./screens/settings";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
});

const TITLES: Record<string, string> = {
  "/": "Dashboard",
  "/tools": "Tools",
  "/plugins": "Plugins",
  "/project": "Project",
  "/operations": "Operations",
  "/settings": "Settings",
};

function Shell() {
  const route = useRoute();
  const status = useStatus();
  const invalidate = useInvalidateAppData();
  const { toast } = useToast();
  const timer = useRef<number | undefined>(undefined);

  const theme = normalizeTheme(status.data?.settings.theme ?? readStoredTheme());

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    const scheduleInvalidate = () => {
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(invalidate, 250);
    };

    const offFinished = Events.On(OperationFinished, (event) => {
      scheduleInvalidate();
      const operation = event.data;
      if (!operation) return;
      const variant =
        operation.status === OperationStatus.StatusSucceeded
          ? "success"
          : operation.status === OperationStatus.StatusFailed
            ? "error"
            : "info";
      if (operation.status !== OperationStatus.StatusCanceled) {
        toast({
          title: operation.title,
          description:
            operation.status === OperationStatus.StatusSucceeded
              ? "Operation finished successfully."
              : operation.error || "Operation failed.",
          variant,
        });
      }
    });
    const offState = Events.On(StateChanged, scheduleInvalidate);
    const offQueued = Events.On(OperationQueued, scheduleInvalidate);
    const offUpdated = Events.On(OperationUpdated, scheduleInvalidate);

    return () => {
      offFinished();
      offState();
      offQueued();
      offUpdated();
      window.clearTimeout(timer.current);
    };
  }, [invalidate, toast]);

  const title = TITLES[route.path] ?? "asdf mise GUI";
  const page = (() => {
    switch (route.path) {
      case "/tools":
        return <Tools />;
      case "/plugins":
        return <Plugins />;
      case "/project":
        return <Project />;
      case "/operations":
        return <Operations selectedID={route.params.get("op")} />;
      case "/settings":
        return <Settings />;
      default:
        return <Dashboard />;
    }
  })();

  return (
    <div className="flex h-full min-h-0 bg-canvas text-ink">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar title={title} />
        <main key={route.path} className="animate-fade-up min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-[1240px] px-7 py-7">{page}</div>
        </main>
        <StatusBar />
      </div>
    </div>
  );
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={350}>
        <ToastProvider>
          <Shell />
        </ToastProvider>
      </TooltipProvider>
    </QueryClientProvider>
  );
}
