import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ManagerService,
  OperationService,
  PluginsService,
  ProjectService,
  SettingsService,
  ToolsService,
  UpdateService,
  WorkspaceService,
} from "./bindings";

export const queryKeys = {
  status: ["status"] as const,
  tools: ["tools"] as const,
  plugins: ["plugins"] as const,
  pluginCatalog: ["plugin-catalog"] as const,
  operations: ["operations"] as const,
  operation: (id: string) => ["operation", id] as const,
  project: (dir: string) => ["project", dir] as const,
  recent: ["recent-projects"] as const,
  settings: ["settings"] as const,
  diagnostics: ["diagnostics"] as const,
  available: (tool: string) => ["available", tool] as const,
  latest: ["latest"] as const,
  comparisons: ["version-comparisons"] as const,
  workspaces: ["workspaces"] as const,
  workspace: (id: string) => ["workspace", id] as const,
  update: ["update"] as const,
  wslDistros: ["wsl-distros"] as const,
};

export function useStatus(refresh = false) {
  return useQuery({
    queryKey: queryKeys.status,
    queryFn: () => ManagerService.Status(refresh),
    staleTime: 10_000,
  });
}

export function useTools() {
  return useQuery({
    queryKey: queryKeys.tools,
    queryFn: () => ToolsService.List(),
    staleTime: 5_000,
  });
}

export function usePlugins() {
  return useQuery({
    queryKey: queryKeys.plugins,
    queryFn: () => PluginsService.List(),
    staleTime: 10_000,
  });
}

export function usePluginCatalog(engine = "auto", enabled = true) {
  return useQuery({
    // asdf and mise publish different registries, so the active engine is part
    // of the cache identity even though the endpoint itself has no arguments.
    queryKey: [...queryKeys.pluginCatalog, engine],
    queryFn: () => PluginsService.Available(),
    staleTime: 24 * 60 * 60 * 1000,
    enabled,
  });
}

export function useOperations() {
  return useQuery({
    queryKey: queryKeys.operations,
    queryFn: () => OperationService.List(),
    staleTime: 2_000,
    refetchInterval: (query) => {
      const data = query.state.data;
      if (!data) return false;
      const active = data.some(
        (operation) =>
          operation.status === "queued" || operation.status === "running" || operation.status === "canceling",
      );
      return active ? 4_000 : false;
    },
  });
}

export function useOperation(id: string | null) {
  return useQuery({
    queryKey: queryKeys.operation(id ?? ""),
    queryFn: () => OperationService.Get(id as string),
    enabled: Boolean(id),
    staleTime: 500,
    refetchInterval: (query) => {
      const status = query.state.data?.operation.status;
      const active = status === "queued" || status === "running" || status === "canceling";
      return active ? 700 : false;
    },
  });
}

export function useProject(dir: string, remember = false, enabled = true) {
  return useQuery({
    queryKey: queryKeys.project(dir),
    queryFn: () => ProjectService.Open(dir, remember),
    enabled,
    staleTime: 5_000,
  });
}

export function useRecentProjects() {
  return useQuery({
    queryKey: queryKeys.recent,
    queryFn: () => ProjectService.Recent(),
    staleTime: 10_000,
  });
}

export function useSettings() {
  return useQuery({
    queryKey: queryKeys.settings,
    queryFn: () => SettingsService.Get(),
    staleTime: 10_000,
  });
}

export function useDiagnostics(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.diagnostics,
    queryFn: () => ManagerService.Diagnostics(),
    staleTime: 15_000,
    enabled,
  });
}

export function useWSLDistros(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.wslDistros,
    queryFn: () => SettingsService.WSLDistros(),
    staleTime: 60_000,
    enabled,
    retry: false,
  });
}

export function useAvailableVersions(tool: string | null) {
  return useQuery({
    queryKey: queryKeys.available(tool ?? ""),
    queryFn: () => ToolsService.Available(tool as string),
    enabled: Boolean(tool),
    staleTime: 5 * 60 * 1000,
    retry: false,
  });
}

export function useLatestVersions() {
  return useQuery({
    queryKey: queryKeys.latest,
    queryFn: () => ToolsService.LatestAll(),
    staleTime: 60_000,
  });
}

export function useVersionComparisons() {
  return useQuery({
    queryKey: queryKeys.comparisons,
    queryFn: () => ToolsService.Compare(),
    staleTime: 60_000,
  });
}

export function useWorkspaces() {
  return useQuery({
    queryKey: queryKeys.workspaces,
    queryFn: () => WorkspaceService.List(),
    staleTime: 5_000,
  });
}

export function useWorkspace(id: string | null) {
  return useQuery({
    queryKey: queryKeys.workspace(id ?? ""),
    queryFn: () => WorkspaceService.Scan(id as string),
    enabled: Boolean(id),
    staleTime: 15_000,
  });
}

export function useUpdateStatus() {
  return useQuery({
    queryKey: queryKeys.update,
    queryFn: () => UpdateService.Status(),
    staleTime: 5_000,
  });
}

export function useCheckForUpdates() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => UpdateService.Check(),
    onSuccess: (status) => {
      queryClient.setQueryData(queryKeys.update, status);
    },
  });
}

export function useInstallUpdate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => UpdateService.Install(),
    onSuccess: (status) => {
      queryClient.setQueryData(queryKeys.update, status);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.update });
    },
  });
}

export function useInvalidateAppData() {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.operations });
    void queryClient.invalidateQueries({ queryKey: queryKeys.status });
    void queryClient.invalidateQueries({ queryKey: queryKeys.tools });
    void queryClient.invalidateQueries({ queryKey: queryKeys.plugins });
    void queryClient.invalidateQueries({ queryKey: ["project"] });
    void queryClient.invalidateQueries({ queryKey: queryKeys.latest });
    void queryClient.invalidateQueries({ queryKey: queryKeys.comparisons });
    void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces });
    void queryClient.invalidateQueries({ queryKey: ["workspace"] });
    void queryClient.invalidateQueries({ queryKey: queryKeys.recent });
  };
}
