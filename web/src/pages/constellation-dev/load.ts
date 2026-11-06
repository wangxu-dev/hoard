import { Exploration } from "@/lib/api";
import type { ExploreScope } from "@mygo";

// View-state only: native code reads one transaction, analyzes, versions and caches.
export async function readCurrentLibrary(signal: AbortSignal, progress: (count: number) => void, clientID: string, scope: ExploreScope = { accountId: "", source: "" }) {
  const task = await Exploration.start(scope, clientID);
  const cancel = () => { void Exploration.cancel(task.id).catch(() => {}); };
  signal.addEventListener("abort", cancel, { once: true });
  try {
    if (signal.aborted) { cancel(); throw new Error("CANCELLED"); }
    for (;;) {
      if (signal.aborted) throw new Error("CANCELLED");
      const status = await Exploration.status(task.id);
      if (status.state === "complete") {
        const snapshot = await Exploration.result(task.id);
        if (!snapshot) throw new Error("SNAPSHOT_MISSING");
        if (signal.aborted) throw new Error("CANCELLED");
        if (snapshot.schemaVersion !== 2 || snapshot.scope.accountId !== scope.accountId || snapshot.scope.source !== scope.source) throw new Error("SNAPSHOT_SCOPE_INVALID");
        console.warn("[interest-dev] native-task", JSON.stringify({ state: status.state, cacheHit: status.cacheHit, schema: snapshot.schemaVersion, analysisVersion: snapshot.analysisVersion, inputVersion: snapshot.inputVersion, persistence: snapshot.meta.persistence, consistency: snapshot.meta.consistency }));
        progress(snapshot.meta.coverage.reduce((n, c) => n + c.worksTotal, 0));
        return snapshot;
      }
      if (["failed", "cancelled", "superseded"].includes(status.state)) throw new Error(status.error || status.state);
      await new Promise<void>((resolve, reject) => {
        const stop = () => { clearTimeout(timer); signal.removeEventListener("abort", stop); reject(new Error("CANCELLED")); };
        const timer = setTimeout(() => { signal.removeEventListener("abort", stop); resolve(); }, 100);
        signal.addEventListener("abort", stop, { once: true });
      });
    }
  } finally { signal.removeEventListener("abort", cancel); }
}
