import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { PublicationTimeline } from "./publication-timeline";
import type { ExploreScope, ExplorationWork } from "@/lib/api";
import type { Scope } from "@/lib/library";
import { libraryScope, useLibraryFilters } from "@/lib/library-filters";
import shell from "./shell.html?raw";
import { Button } from "@/components/ui/button";
import { ContentDetails } from "./details";
import type { PanelView } from "./view-types";
import { onLibraryChanged } from "@/lib/api";
import { readCurrentLibrary } from "./load";
import { mountInterestSky, prepareExploration, type ViewControls } from "./renderer";
import { ExploreScopeCache, libraryChangeAffectsScope } from "./scope-cache";
import { ArrowLeft, LocateFixed, Maximize2, Minus, Pause, Play, Plus } from "lucide-react";
import "./styles.css";

// Last good snapshots per scope are a return-transition surface, not a replacement for
// native validation. Every entry revalidates; unchanged revisions do not remount.
type LibrarySnapshot = Awaited<ReturnType<typeof readCurrentLibrary>>;
type PreparedExploration = Awaited<ReturnType<typeof prepareExploration>>;
const scopeCache = new ExploreScopeCache<LibrarySnapshot, PreparedExploration>(4);
let preparationTask: Promise<{ key: string; model: PreparedExploration }> | null = null;
let preloadTask: { scopeKey: string; promise: Promise<{ snapshot: LibrarySnapshot; model: PreparedExploration }> } | null = null;
const currentExploreScope = (scope?: Scope): ExploreScope => ({ accountId: scope?.accountId ?? "", source: scope?.source ?? "" });
const scopeKeyOf = (scope: ExploreScope) => `${scope.accountId}:${scope.source}`;
const snapshotKey = (snapshot: LibrarySnapshot) => `${snapshot.scope.accountId}:${snapshot.scope.source}:${snapshot.inputRevision}:${snapshot.analysisVersion}`;

function prepareSnapshot(snapshot: LibrarySnapshot): Promise<PreparedExploration> {
  const key = snapshotKey(snapshot);
  const cached = scopeCache.get(scopeKeyOf(snapshot.scope));
  if (cached?.key === key) return Promise.resolve(cached.model);
  if (preparationTask) return preparationTask.then((result) => result.key === key ? result.model : prepareSnapshot(snapshot));
  const task = prepareExploration(snapshot).then((model) => ({ key, model }));
  preparationTask = task;
  void task.finally(() => { if (preparationTask === task) preparationTask = null; }).catch(() => {});
  return task.then((result) => result.model);
}

export async function preloadExploration(signal: AbortSignal) {
  const exploreScope = currentExploreScope(libraryScope());
  const scopeKey = scopeKeyOf(exploreScope);
  const cached = scopeCache.get(scopeKey);
  if (signal.aborted) return;
  if (preloadTask?.scopeKey === scopeKey) return preloadTask.promise;
  const promise = readCurrentLibrary(signal, () => {}, crypto.randomUUID(), exploreScope, cached?.snapshot).then(async (snapshot) => {
    const model = await prepareSnapshot(snapshot);
    if (!signal.aborted) scopeCache.set(scopeKey, { snapshot, model, key: snapshotKey(snapshot) });
    return { snapshot, model };
  });
  const entry = { scopeKey, promise };
  preloadTask = entry;
  try { await promise; }
  finally { if (preloadTask === entry) preloadTask = null; }
}

export default function ConstellationDevPage() {
  const { t } = useTranslation("explore");
  const { scope } = useLibraryFilters();
  const exploreScope = currentExploreScope(scope);
  const scopeKey = scopeKeyOf(exploreScope);
  const host = useRef<HTMLDivElement>(null);
  const client = useRef(crypto.randomUUID());
  const [attempt, setAttempt] = useState(0), [error, setError] = useState("");
  const [timelineWorks, setTimelineWorks] = useState<ExplorationWork[]>([]);
  const [panel, setPanel] = useState<PanelView | null>(null);
  const [ready, setReady] = useState(false);
  const [controls, setControls] = useState<ViewControls | null>(null);
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const stop = onLibraryChanged((change) => {
      if (!libraryChangeAffectsScope(change.accountId, exploreScope)) return;
      if (timer !== null) clearTimeout(timer);
      timer = setTimeout(() => {
        timer = null;
        setAttempt((n) => n + 1);
      }, 1500);
    });
    return () => {
      stop();
      if (timer !== null) clearTimeout(timer);
    };
  }, [exploreScope.accountId, exploreScope.source]);
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
    const cached = scopeCache.get(scopeKey);
    if (cached) {
      queueMicrotask(() => paint(cached.snapshot, cached.model));
    }
    const load = (preloadTask?.scopeKey === scopeKey ? preloadTask.promise : null) ?? Promise.resolve().then(() => {
      if (abort.signal.aborted) throw new Error("CANCELLED");
      return readCurrentLibrary(abort.signal, () => {}, client.current, exploreScope, cached?.snapshot);
    }).then(async (snapshot) => ({ snapshot, model: await prepareSnapshot(snapshot) }));
    void load.then(({ snapshot, model }) => {
      if (abort.signal.aborted || !host.current) return;
      if (!painted || scopeKeyOf(painted.scope) !== scopeKeyOf(snapshot.scope) || painted.inputRevision !== snapshot.inputRevision || painted.analysisVersion !== snapshot.analysisVersion) paint(snapshot, model);
      scopeCache.set(scopeKey, { snapshot, model, key: snapshotKey(snapshot) });
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
  }, [attempt, exploreScope.accountId, exploreScope.source, mountInterestSky]);
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
