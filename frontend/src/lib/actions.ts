import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { Operation } from "./bindings";
import { errorMessage } from "./format";
import { queryKeys } from "./query";
import { useToast } from "../components/ui/toast";

type QueueInput = {
  run: () => Promise<Operation>;
};

// useQueueOperation centralizes optimistic feedback for every asdf mutation.
// The queue itself is the source of truth; this only surfaces "queued" early so
// the UI feels responsive before the operation actually starts.
export function useQueueOperation() {
  const queryClient = useQueryClient();
  const { toast } = useToast();

  return useMutation({
    mutationFn: (input: QueueInput) => input.run(),
    onSuccess: (operation) => {
      toast({
        title: "Operation queued",
        description: operation.title,
        variant: "info",
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.operations });
    },
    onError: (error) => {
      toast({
        title: "Could not queue operation",
        description: errorMessage(error),
        variant: "error",
      });
    },
  });
}

export function useRefreshAll() {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries();
  };
}
