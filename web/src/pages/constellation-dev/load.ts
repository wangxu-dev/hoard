import { App, Exploration } from "@/lib/api";
import type { ExploreScope, ExplorationDiagnostic } from "@mygo";
import { diagnosticError } from "./diagnostics";
let sequence=0;

// View-state only: native code reads one transaction, analyzes, versions and caches.
export async function readCurrentLibrary(signal: AbortSignal, progress: (count: number) => void, clientID: string, scope: ExploreScope = { accountId: "", source: "" }) {
  const report=(stage:string,error?:unknown,state="")=>{
    const data:ExplorationDiagnostic={stage,source:scope.source,accountScoped:!!scope.accountId,sequence:request,...diagnosticError(error),state};
    console.warn("[hoard:exploration-ui]",JSON.stringify(data));
    void App.reportExplorationDiagnostic(data).catch(error=>console.warn("[hoard:exploration-ui] native-log-unavailable",JSON.stringify(diagnosticError(error))));
  };
  const request=++sequence;report("request");report("start-call");
  let task;
  try {task=await Exploration.start(scope,clientID);report("start-ok");}catch(error){report("start-reject",error);throw error;}
  let previousState="";
  const cancel = () => { report("abort");void Exploration.cancel(task.id).catch(error=>report("failed",error)); };
  signal.addEventListener("abort", cancel, { once: true });
  try {
    if (signal.aborted) { report("late-drop");cancel(); throw new Error("CANCELLED"); }
    for (;;) {
      if (signal.aborted) throw new Error("CANCELLED");
      if(!previousState)report("status-call");
      let status;
      try{status=await Exploration.status(task.id);}catch(error){report("status-reject",error);throw error;}
      if(status.state!==previousState){previousState=status.state;report("status-state",undefined,status.state);}
      if (status.state === "complete") {
        report("result-call");
        let snapshot;
        try{snapshot=await Exploration.result(task.id);}catch(error){report("result-reject",error);throw error;}
        if (!snapshot) throw new Error("SNAPSHOT_MISSING");
        if (signal.aborted) {report("late-drop");throw new Error("CANCELLED");}
        if (snapshot.schemaVersion !== 2 || snapshot.scope.accountId !== scope.accountId || snapshot.scope.source !== scope.source) {report("scope-reject",new Error("SNAPSHOT_SCOPE_INVALID"));throw new Error("SNAPSHOT_SCOPE_INVALID");}
        report("result-ok");
        console.warn("[interest-dev] native-task", JSON.stringify({ state: status.state, cacheHit: status.cacheHit, schema: snapshot.schemaVersion, analysisVersion: snapshot.analysisVersion, inputVersion: snapshot.inputVersion, persistence: snapshot.meta.persistence, consistency: snapshot.meta.consistency }));
        progress(snapshot.meta.coverage.reduce((n, c) => n + c.worksTotal, 0));
        return snapshot;
      }
      if (["failed", "cancelled", "superseded"].includes(status.state)) {report("failed",new Error(status.error||status.state),status.state);throw new Error(status.error || status.state);}
      await new Promise<void>((resolve, reject) => {
        const stop = () => { clearTimeout(timer); signal.removeEventListener("abort", stop); reject(new Error("CANCELLED")); };
        const timer = setTimeout(() => { signal.removeEventListener("abort", stop); resolve(); }, 100);
        signal.addEventListener("abort", stop, { once: true });
      });
    }
  } finally { signal.removeEventListener("abort", cancel); }
}
