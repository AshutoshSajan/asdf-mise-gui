import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Button, type ButtonProps } from "./ui/button";
import { useToast } from "./ui/toast";
import { cn } from "../lib/utils";

export function CopyButton({
  value,
  label = "Copy",
  className,
  variant = "ghost",
  size = "sm",
}: {
  value: string;
  label?: string;
  className?: string;
  variant?: ButtonProps["variant"];
  size?: ButtonProps["size"];
}) {
  const { toast } = useToast();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      toast({ title: "Could not copy", description: "Clipboard access was denied.", variant: "error" });
    }
  };

  return (
    <Button variant={variant} size={size} className={cn("gap-1.5", className)} onClick={copy}>
      {copied ? <Check className="size-3.5 text-accent" /> : <Copy className="size-3.5" />}
      {size === "icon" || size === "icon-sm" ? null : copied ? "Copied" : label}
    </Button>
  );
}
