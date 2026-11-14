import * as React from "react";
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from "lucide-react";
import { cn } from "../../lib/utils";

type ToastVariant = "success" | "error" | "info" | "warning";

type ToastInput = {
  title: string;
  description?: string;
  variant?: ToastVariant;
};

type ToastItem = ToastInput & { id: number; variant: ToastVariant };

type ToastContextValue = {
  toast: (input: ToastInput) => void;
};

const ToastContext = React.createContext<ToastContextValue | null>(null);

let nextToastID = 1;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<ToastItem[]>([]);

  const remove = React.useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);

  const toast = React.useCallback(
    (input: ToastInput) => {
      const id = nextToastID++;
      const item: ToastItem = { ...input, id, variant: input.variant ?? "info" };
      setItems((current) => [...current.slice(-3), item]);
      window.setTimeout(() => remove(id), 5200);
    },
    [remove],
  );

  const value = React.useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed bottom-11 right-5 z-[80] flex w-80 flex-col gap-2">
        {items.map((item) => (
          <div
            key={item.id}
            className={cn(
              "animate-toast pointer-events-auto flex items-start gap-3 rounded-xl border bg-panel p-3.5 shadow-xl shadow-black/35",
              item.variant === "success" && "border-accent/25",
              item.variant === "error" && "border-danger/30",
              item.variant === "warning" && "border-warn/30",
              item.variant === "info" && "border-line",
            )}
          >
            <ToastIcon variant={item.variant} />
            <div className="min-w-0 flex-1">
              <p className="text-[13px] font-medium text-ink">{item.title}</p>
              {item.description ? (
                <p className="mt-0.5 text-xs leading-relaxed text-muted">{item.description}</p>
              ) : null}
            </div>
            <button
              className="rounded p-0.5 text-faint transition hover:text-ink"
              onClick={() => remove(item.id)}
              aria-label="Dismiss"
            >
              <X className="size-3.5" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastIcon({ variant }: { variant: ToastVariant }) {
  const className = "mt-0.5 size-4 shrink-0";
  switch (variant) {
    case "success":
      return <CheckCircle2 className={cn(className, "text-accent")} />;
    case "error":
      return <XCircle className={cn(className, "text-danger")} />;
    case "warning":
      return <AlertTriangle className={cn(className, "text-warn")} />;
    default:
      return <Info className={cn(className, "text-info")} />;
  }
}

export function useToast() {
  const context = React.useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used inside ToastProvider");
  }
  return context;
}
