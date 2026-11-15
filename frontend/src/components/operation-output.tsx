import { useEffect, useRef, useState } from "react";
import type { OutputLine } from "../lib/bindings";
import { cn } from "../lib/utils";

export function OperationOutput({
  lines,
  live,
  className,
}: {
  lines: OutputLine[];
  live?: boolean;
  className?: string;
}) {
  const scroller = useRef<HTMLDivElement>(null);
  const [follow, setFollow] = useState(true);

  useEffect(() => {
    const element = scroller.current;
    if (!element || !follow) return;
    element.scrollTop = element.scrollHeight;
  }, [lines, follow]);

  const onScroll = () => {
    const element = scroller.current;
    if (!element) return;
    const distance = element.scrollHeight - element.scrollTop - element.clientHeight;
    setFollow(distance < 48);
  };

  return (
    <div className={cn("relative overflow-hidden rounded-xl border border-line bg-[#080b09]", className)}>
      <div
        ref={scroller}
        onScroll={onScroll}
        className="max-h-[460px] min-h-40 overflow-auto py-3 font-mono text-[11.5px] leading-5"
      >
        {lines.length === 0 ? (
          <div className="flex h-40 flex-col items-center justify-center gap-1.5 text-center">
            <span className="text-faint">No output yet.</span>
            <span className="text-[10px] text-faint/70">
              {live ? "Lines will appear here as the command writes them." : "This operation did not write any output."}
            </span>
          </div>
        ) : (
          lines.map((line, index) => (
            <div key={index} className="log-line group">
              <span className="select-none text-right text-[10px] text-faint/50">{index + 1}</span>
              <span
                className={cn(
                  "whitespace-pre-wrap break-words",
                  line.stream === "stderr" ? "text-danger/90" : "text-[#c9d4cc]",
                )}
              >
                {line.text || " "}
              </span>
            </div>
          ))
        )}
      </div>
      {lines.length > 0 ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-0 flex justify-center bg-gradient-to-t from-[#080b09] via-[#080b09]/85 to-transparent pb-2 pt-6">
          <button
            type="button"
            onClick={() => setFollow(true)}
            className={cn(
              "pointer-events-auto rounded-full border border-line bg-elevated px-3 py-1 text-[10px] text-muted transition hover:text-ink",
              follow && "pointer-events-none opacity-0",
            )}
          >
            Jump to latest
          </button>
        </div>
      ) : null}
    </div>
  );
}
