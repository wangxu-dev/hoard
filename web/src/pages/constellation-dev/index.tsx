import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { PublicationTimeline } from "./publication-timeline";
import type { ExplorationWork } from "@/lib/api";
import shell from "./shell.html?raw";
import { Button } from "@/components/ui/button";
import { ContentDetails } from "./details";
import type { PanelView } from "./view-types";
import { onLibraryChanged } from "@/lib/api";
import { readCurrentLibrary } from "./load";
import { mountInterestSky, prepareExploration, type ViewControls } from "./renderer.js";
import { ArrowLeft, LocateFixed, Maximize2, Minus, Pause, Play, Plus } from "lucide-react";
import "./styles.css";

// Last good union snapshot is a return-transition surface, not a replacement for
// native validation. Every entry revalidates; unchanged revisions do not remount.
type LibrarySnapshot = Awaited<ReturnType<typeof readCurrentLibrary>>;
type PreparedExploration = Awaited<ReturnType<typeof prepareExploration>>;
let returnSnapshot: LibrarySnapshot | null = null;
let returnPrepared: { key: string; model: PreparedExploration } | null = null;
let preparationTask: Promise<{ key: string; model: PreparedExploration }> | null = null;
let preloadTask: Promise<{ snapshot: LibrarySnapshot; model: PreparedExploration }> | null = null;
const snapshotKey = (snapshot: LibrarySnapshot) => `${snapshot.scope.accountId}:${snapshot.scope.source}:${snapshot.inputRevision}:${snapshot.analysisVersion}`;

function prepareSnapshot(snapshot: LibrarySnapshot): Promise<PreparedExploration> {
  const key = snapshotKey(snapshot);
  if (returnPrepared?.key === key) return Promise.resolve(returnPrepared.model);
  if (preparationTask) return preparationTask.then((result) => result.key === key ? result.model : prepareSnapshot(snapshot));
  const task = prepareExploration(snapshot).then((model) => {
    returnPrepared = { key, model };
    return { key, model };
  });
  preparationTask = task;
  void task.finally(() => { if (preparationTask === task) preparationTask = null; }).catch(() => {});
  return task.then((result) => result.model);
}

export async function preloadExploration(signal: AbortSignal) {
  if ((returnSnapshot && returnPrepared?.key === snapshotKey(returnSnapshot)) || signal.aborted) return;
  if (preloadTask) return preloadTask;
  const task = readCurrentLibrary(signal, () => {}, crypto.randomUUID()).then(async (snapshot) => {
    const model = await prepareSnapshot(snapshot);
    if (!signal.aborted) returnSnapshot = snapshot;
    return { snapshot, model };
  });
  preloadTask = task;
  try { await task; }
  finally { if (preloadTask === task) preloadTask = null; }
}

export default function ConstellationDevPage() {
  const { t } = useTranslation("explore");
  const host = useRef<HTMLDivElement>(null);
  const client = useRef(crypto.randomUUID());
  const [attempt, setAttempt] = useState(0), [error, setError] = useState("");
  const [timelineWorks, setTimelineWorks] = useState<ExplorationWork[]>([]);
  const [panel, setPanel] = useState<PanelView | null>(null);
  const [ready, setReady] = useState(false);
  const [controls, setControls] = useState<ViewControls | null>(null);
  const [paused, setPaused] = useState(false);
  useEffect(() => onLibraryChanged(() => setAttempt((n) => n + 1)), []);
  useEffect(() => {
    const abort = new AbortController(); let dispose: (() => void) | undefined;
    setReady(false); setError(""); setPanel(null); setPaused(false);
    let painted: LibrarySnapshot | null = null;
    const paint = (snapshot: LibrarySnapshot, model: PreparedExploration) => {
      if (abort.signal.aborted || !host.current) return;
      dispose?.(); painted = null; setReady(false);
      host.current.innerHTML = shell;
      dispose = mountInterestSky(host.current, snapshot, model, { onPanel: setPanel, onControls: (next) => { setControls(next); if (next) setPaused(next.isPaused()); } });
      painted = snapshot;
      setTimelineWorks(snapshot.scopes.flatMap(scope => scope.works));
      setReady(true);
    };
    if (returnSnapshot && returnPrepared?.key === snapshotKey(returnSnapshot)) {
      const cachedSnapshot = returnSnapshot, cachedModel = returnPrepared.model;
      queueMicrotask(() => paint(cachedSnapshot, cachedModel));
    }
    const load = preloadTask ?? Promise.resolve().then(() => {
      if (abort.signal.aborted) throw new Error("CANCELLED");
      return readCurrentLibrary(abort.signal, () => {}, client.current);
    }).then(async (snapshot) => ({ snapshot, model: await prepareSnapshot(snapshot) }));
    void load.then(({ snapshot, model }) => {
      if (abort.signal.aborted || !host.current) return;
      if (!painted || painted.inputRevision !== snapshot.inputRevision || painted.analysisVersion !== snapshot.analysisVersion) paint(snapshot, model);
      returnSnapshot = snapshot;
      // Aggregate counters only, no original IDs/titles or media URLs in diagnostics.
      console.warn("[interest-dev] coverage", JSON.stringify(snapshot.meta.coverage));
      setReady(true);
    }).catch((err: unknown) => {
      if (abort.signal.aborted) return;
      const safeCode = err instanceof Error ? err.message.match(/(?:STORE_NOT_READY|INVALID_REQUEST|TASK_[A-Z_]+|INVALID_SCOPE|SCOPE_MISMATCH|SNAPSHOT_[A-Z_]+|EXPLORATION_FAILED)/)?.[0] || "BRIDGE_OR_TRANSPORT" : "UNKNOWN";
      console.warn("[interest-dev] native-error", JSON.stringify({ code: safeCode }));
      if (!painted) setReady(false);
      setError(painted ? "refresh" : "load");
    });
    return () => { abort.abort(); dispose?.(); };
  }, [attempt, mountInterestSky]);
  return <div id="interest-native-root"><div ref={host} />{ready && <>
    <div className="explore-controls explore-glass" role="toolbar" aria-label={t("controls.toolbar")}>
      <Button variant="ghost" size="icon" onClick={() => controls?.back()} disabled={!panel} aria-label={t("controls.back")} title={t("controls.back")}><ArrowLeft /></Button>
      <Button variant="ghost" size="icon" onClick={() => controls?.reset()} aria-label={t("controls.reset")} title={t("controls.reset")}><Maximize2 /></Button>
      <span className="explore-nav-divider" />
      <Button variant="ghost" size="icon" onClick={() => controls?.zoomOut()} aria-label={t("controls.zoomOut")} title={t("controls.zoomOut")}><Minus /></Button>
      <Button variant="ghost" size="icon" onClick={() => controls?.zoomIn()} aria-label={t("controls.zoomIn")} title={t("controls.zoomIn")}><Plus /></Button>
      <Button variant="ghost" size="icon" onClick={() => controls && setPaused(controls.toggleRotation())} aria-label={t(paused ? "controls.resume" : "controls.pause")} title={t(paused ? "controls.resume" : "controls.pause")} aria-pressed={paused}>{paused ? <Play /> : <Pause />}</Button>
      <Button variant="ghost" size="icon" onClick={() => controls?.focusSelected()} disabled={!panel?.work} aria-label={t("controls.focus")} title={t("controls.focus")}><LocateFixed /></Button>
    </div>
    {error && <div className="explore-refresh-warning explore-glass" role="status">{t(`error.${error}`)}<Button variant="ghost" size="sm" onClick={() => setAttempt(n => n + 1)}>{t("retry")}</Button></div>}
    {controls && <PublicationTimeline works={timelineWorks} controls={controls} />}
    <ContentDetails view={panel} />
  </>}{!ready && <div className="native-loading" role={error ? "alert" : "status"}>
    <span>{error ? t(`error.${error}`) : t("loading")}</span>
    {error && <Button variant="outline" onClick={() => setAttempt((n) => n + 1)}>{t("retry")}</Button>}
  </div>}</div>;
}
