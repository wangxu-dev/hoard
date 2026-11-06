import { useEffect, useRef, useState } from "react";
import shell from "./shell.html?raw";
import { Button } from "@/components/ui/button";
import { ContentDetails } from "./details";
import type { PanelView } from "./view-types";
import { onLibraryChanged } from "@/lib/api";
import { readCurrentLibrary } from "./load";
import { mountInterestSky } from "./renderer.js";
import "./styles.css";

export default function ConstellationDevPage() {
  const host = useRef<HTMLDivElement>(null);
  const client = useRef(crypto.randomUUID());
  const [attempt, setAttempt] = useState(0), [error, setError] = useState("");
  const [panel, setPanel] = useState<PanelView | null>(null);
  const [ready, setReady] = useState(false);
  useEffect(() => onLibraryChanged(() => setAttempt((n) => n + 1)), []);
  useEffect(() => {
    const abort = new AbortController(); let dispose: (() => void) | undefined;
    setReady(false); setError(""); setPanel(null);
    void readCurrentLibrary(abort.signal, () => {}, client.current).then((snapshot) => {
      if (abort.signal.aborted || !host.current) return;
      host.current.innerHTML = shell;
      dispose = mountInterestSky(host.current, snapshot, { onPanel: setPanel });
      // Aggregate counters only, no original IDs/titles or media URLs in diagnostics.
      console.warn("[interest-dev] coverage", JSON.stringify(snapshot.meta.coverage));
      setReady(true);
    }).catch((err: unknown) => {
      if (abort.signal.aborted) return;
      const safeCode = err instanceof Error ? err.message.match(/(?:STORE_NOT_READY|INVALID_REQUEST|TASK_[A-Z_]+|INVALID_SCOPE|SCOPE_MISMATCH|SNAPSHOT_[A-Z_]+|EXPLORATION_FAILED)/)?.[0] || "BRIDGE_OR_TRANSPORT" : "UNKNOWN";
      console.warn("[interest-dev] native-error", JSON.stringify({ code: safeCode }));
      setError("暂时无法读取收藏");
    });
    return () => { abort.abort(); dispose?.(); };
  }, [attempt, mountInterestSky]);
  return <div id="interest-native-root"><div ref={host} />{ready && <ContentDetails view={panel} />}{!ready && <div className="native-loading" role={error ? "alert" : "status"}>
    <span>{error || "正在读取收藏…"}</span>
    {error && <Button variant="outline" onClick={() => setAttempt((n) => n + 1)}>重试</Button>}
  </div>}</div>;
}
