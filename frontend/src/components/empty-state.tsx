import * as React from "react";
import { cn } from "../lib/utils";
import { Card } from "./ui/card";

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center px-6 py-12 text-center", className)}>
      <div className="mb-4 grid size-11 place-items-center rounded-xl border border-line bg-elevated text-muted">
        {icon}
      </div>
      <h3 className="text-[13px] font-medium text-ink">{title}</h3>
      {description ? <div className="mt-1.5 max-w-sm text-xs leading-relaxed text-muted">{description}</div> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}

export function InlineEmpty({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn("border-dashed bg-transparent px-5 py-6 text-center text-xs text-muted", className)}>
      {children}
    </Card>
  );
}
