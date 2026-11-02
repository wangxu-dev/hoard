import { useEffect } from "react";
import { Progress } from "@/components/ui/progress";
import type { SyncState } from "@/lib/library";
// Only real progress is drawn: a running list with a known remote total.
// The line stays mounted and fades, so list turns and tab switches never flash it.
export function ProgressLine({ state: current }: { state?: SyncState; states: SyncState[] }) {
  // Follows the pill: the selected list while it is syncing with a known total.
  const state = current && current.kind !== "cover" && (current.status === "running" || current.status === "queued") ? current : undefined;
  const total = state?.remoteTotal ?? 0;
  const shown = !!state && total > 0;
  const value = state && shown ? Math.min(100, (state.items / total) * 100) : 0;
  useEffect(() => { console.debug("[hoard:ui] progress line", { shown, kind: state?.kind, status: state?.status }); }, [shown, state?.kind, state?.status]);
  return <div data-control="progress" data-shown={shown} aria-hidden={!shown} className={`pointer-events-none absolute inset-x-0 bottom-0 h-0.5 overflow-hidden transition-opacity duration-300 ${shown ? "opacity-100" : "opacity-0"}`}>
    <Progress value={value} className="h-0.5 rounded-none bg-transparent [&>[data-slot=progress-indicator]]:bg-muted-foreground/40" />
  </div>;
}
