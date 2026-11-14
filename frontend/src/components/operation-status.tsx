import * as React from "react";
import { Ban, CheckCircle2, CircleDashed, Clock3, Loader2, XCircle } from "lucide-react";
import { Badge } from "./ui/badge";
import { cn } from "../lib/utils";
import { OperationStatus, type Operation } from "../lib/bindings";

export function OperationStatusBadge({ status }: { status: OperationStatus | string }) {
  switch (status) {
    case OperationStatus.StatusQueued:
      return (
        <Badge variant="outline" className="gap-1.5">
          <Clock3 className="size-3" />
          Queued
        </Badge>
      );
    case OperationStatus.StatusRunning:
      return (
        <Badge variant="info" className="gap-1.5">
          <Loader2 className="size-3 animate-spin" />
          Running
        </Badge>
      );
    case OperationStatus.StatusCanceling:
      return (
        <Badge variant="warning" className="gap-1.5">
          <Loader2 className="size-3 animate-spin" />
          Canceling
        </Badge>
      );
    case OperationStatus.StatusSucceeded:
      return (
        <Badge variant="success" className="gap-1.5">
          <CheckCircle2 className="size-3" />
          Succeeded
        </Badge>
      );
    case OperationStatus.StatusFailed:
      return (
        <Badge variant="danger" className="gap-1.5">
          <XCircle className="size-3" />
          Failed
        </Badge>
      );
    case OperationStatus.StatusCanceled:
      return (
        <Badge variant="outline" className="gap-1.5">
          <Ban className="size-3" />
          Canceled
        </Badge>
      );
    default:
      return <Badge variant="outline">{status}</Badge>;
  }
}

export function OperationStatusIcon({ status, className }: { status: OperationStatus | string; className?: string }) {
  const base = cn("size-4 shrink-0", className);
  switch (status) {
    case OperationStatus.StatusRunning:
    case OperationStatus.StatusCanceling:
      return <Loader2 className={cn(base, "animate-spin text-info")} />;
    case OperationStatus.StatusSucceeded:
      return <CheckCircle2 className={cn(base, "text-accent")} />;
    case OperationStatus.StatusFailed:
      return <XCircle className={cn(base, "text-danger")} />;
    case OperationStatus.StatusCanceled:
      return <Ban className={cn(base, "text-faint")} />;
    default:
      return <CircleDashed className={cn(base, "text-faint")} />;
  }
}

export function isActiveOperation(operation: Operation): boolean {
  return (
    operation.status === OperationStatus.StatusQueued ||
    operation.status === OperationStatus.StatusRunning ||
    operation.status === OperationStatus.StatusCanceling
  );
}
