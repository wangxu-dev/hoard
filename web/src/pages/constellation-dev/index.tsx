import { useEffect, useRef, useState } from "react";
import shell from "./shell.html?raw";
import { Button } from "@/components/ui/button";
import { ContentDetails } from "./details";
import type { PanelView } from "./view-types";
import { onLibraryChanged } from "@/lib/api";
import { readCurrentLibrary } from "./load";
import { mountInterestSky, type ViewControls } from "./renderer.js";
import { ArrowLeft, LocateFixed, Maximize2, Minus, Pause, Play, Plus } from "lucide-react";
import "./styles.css";

export default function ConstellationDevPage() {
  const host = useRef<HTMLDivElement>(null);
  const client = useRef(crypto.randomUUID());
  const [attempt, setAttempt] = useState(0), [error, setError] = useState("");
  const [panel, setPanel] = useState<PanelView | null>(null);
  const [ready, setReady] = useState(false);
  const [controls, setControls] = useState<ViewControls | null>(null);
  const [paused, setPaused] = useState(false);
  useEffect(() => onLibraryChanged(() => setAttempt((n) => n + 1)), []);
  useEffect(() => {
    const abort = new AbortController(); let dispose: (() => void) | undefined;
    setReady(false); setError(""); setPanel(null); setPaused(false);
    void readCurrentLibrary(abort.signal, () => {}, client.current).then((snapshot) => {
      if (abort.signal.aborted || !host.current) return;
      host.current.innerHTML = shell;
      dispose = mountInterestSky(host.current, snapshot, { onPanel: setPanel, onControls: setControls });
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
  return <div id="interest-native-root"><div ref={host} />{ready && <>
    <div className="explore-controls explore-glass" role="toolbar" aria-label="星系视图">
      <Button variant="ghost" size="icon" onClick={() => controls?.back()} disabled={!panel} aria-label="返回上一视图" title="返回上一视图"><ArrowLeft /></Button>
      <Button variant="ghost" size="icon" onClick={() => controls?.reset()} aria-label="回到全景" title="回到全景"><Maximize2 /></Button>
      <span className="explore-nav-divider" />
      <Button variant="ghost" size="icon" onClick={() => controls?.zoomOut()} aria-label="缩小" title="缩小"><Minus /></Button>
      <Button variant="ghost" size="icon" onClick={() => controls?.zoomIn()} aria-label="放大" title="放大"><Plus /></Button>
      <Button variant="ghost" size="icon" onClick={() => controls && setPaused(controls.toggleRotation())} aria-label={paused ? "继续旋转" : "暂停旋转"} title={paused ? "继续旋转" : "暂停旋转"} aria-pressed={paused}>{paused ? <Play /> : <Pause />}</Button>
      <Button variant="ghost" size="icon" onClick={() => controls?.focusSelected()} disabled={!panel?.work} aria-label="聚焦选中作品" title="聚焦选中作品"><LocateFixed /></Button>
    </div>
    <ContentDetails view={panel} />
  </>}{!ready && <div className="native-loading" role={error ? "alert" : "status"}>
    <span>{error || "正在读取收藏…"}</span>
    {error && <Button variant="outline" onClick={() => setAttempt((n) => n + 1)}>重试</Button>}
  </div>}</div>;
}
