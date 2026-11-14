import { cn } from "../lib/utils";

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" fill="none" className={cn("size-8", className)} aria-hidden="true">
      <rect width="64" height="64" rx="15" className="fill-elevated" />
      <rect x="0.75" y="0.75" width="62.5" height="62.5" rx="14.25" className="stroke-line" strokeWidth="1.5" />
      <path
        d="M19 23.5L28 32L19 40.5"
        className="stroke-accent"
        strokeWidth="4.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M31.5 42H45" className="stroke-ink" strokeWidth="4.2" strokeLinecap="round" />
      <circle cx="45.5" cy="21.5" r="3" className="fill-accent" />
    </svg>
  );
}
