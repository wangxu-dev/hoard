import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "motion/react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronUp, Maximize, Minimize, Music2, PanelRightClose, PanelRightOpen, Pause, Play, RotateCw, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { TooltipProvider } from "@/components/ui/tooltip";
import { getLibrarySource, type LibraryEntry } from "@/lib/library";
import { warmImage, type ListCursor } from "@/lib/library-cursor";
import { setSound, togglePlay, useMediaState, useSound, pauseAll } from "@/lib/player";
import { ControlButton, SeekBar, SpeedMenu, TimeLabel, VolumeControl } from "./controls";
import { VideoStage } from "./video-stage";
import { GalleryStage, type GalleryHandle } from "./gallery-stage";
import { InfoPanel } from "./info-panel";
import { useMedia } from "./use-media";
import { prefetchMedia } from "@/lib/query";

const IDLE_MS = 2000;
const WIDE = 1100;
const slideEase = [0.16, 1, 0.3, 1] as const;
const slide = {
  enter: (dir: number) => ({ y: dir > 0 ? "100%" : dir < 0 ? "-100%" : 0, opacity: dir === 0 ? 0 : 1 }),
  center: { y: 0, opacity: 1 },
  exit: (dir: number) => ({ y: dir > 0 ? "-100%" : "100%", opacity: 1 }),
};

function useWide() {
  const [wide, setWide] = useState(() => window.innerWidth >= WIDE);
  useEffect(() => { const update = () => setWide(window.innerWidth >= WIDE); window.addEventListener("resize", update); return () => window.removeEventListener("resize", update); }, []);
  return wide;
}

function fullscreenElement(): Element | null {
  const doc = document as Document & { webkitFullscreenElement?: Element | null };
  return document.fullscreenElement ?? doc.webkitFullscreenElement ?? null;
}

function toggleFullscreen(element: HTMLElement | null) {
  if (!element) return;
  const doc = document as Document & { webkitExitFullscreen?: () => void };
  const el = element as HTMLElement & { webkitRequestFullscreen?: () => void };
  if (fullscreenElement()) { if (document.exitFullscreen) void document.exitFullscreen().catch(() => undefined); else doc.webkitExitFullscreen?.(); return; }
  if (el.requestFullscreen) void el.requestFullscreen().catch(() => undefined); else el.webkitRequestFullscreen?.();
}

const infoKey = "hoard.viewer.info";

/** Viewer is the immersive, full-window player for videos and galleries. */
export function Viewer({ cursor, start, onClose }: { cursor: ListCursor; start: number; onClose: (index: number) => void }) {
  const { t } = useTranslation("library");
  const wide = useWide();
  const root = useRef<HTMLDivElement>(null);
  const [index, setIndex] = useState(start);
  const [entry, setEntry] = useState<LibraryEntry | undefined>(() => cursor.peek(start));
  const [dir, setDir] = useState(0);
  const [total, setTotal] = useState(cursor.total);
  const [element, setElement] = useState<HTMLMediaElement | null>(null);
  const [gallery, setGallery] = useState<GalleryHandle | null>(null);
  const [badgeSlot, setBadgeSlot] = useState<HTMLElement | null>(null);
  const [slideInfo, setSlideInfo] = useState({ index: 0, count: 1 });
  const [chrome, setChrome] = useState(true);
  const [overControls, setOverControls] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [info, setInfo] = useState(() => localStorage.getItem(infoKey) !== "0");
  const [sheet, setSheet] = useState(false);
  const moving = useRef(false);
  const state = useMediaState(element);
  const sound = useSound();
  const { media, error, loading, refresh, retry } = useMedia(entry);

  useEffect(() => { pauseAll(); }, []);
  useEffect(() => { if (!entry) void cursor.at(start).then(setEntry); }, []);
  useEffect(() => { localStorage.setItem(infoKey, info ? "1" : "0"); }, [info]);
  useEffect(() => {
    // Warm the covers around the current item so the next swipe starts on a picture.
    for (const n of [index + 1, index - 1, index + 2]) void cursor.at(n).then((next) => warmImage(next?.cover)).catch(() => undefined);
  }, [index, cursor]);

  // Once the current item is ready, the next two and the previous one are resolved
  // in that order, and the opening bytes of their videos are loaded.
  useEffect(() => {
    if (loading) return;
    let live = true;
    const timer = setTimeout(async () => {
      for (const n of [index + 1, index + 2, index - 1]) {
        const next = await cursor.at(n).catch(() => undefined);
        if (!live) return;
        if (!next || next.kind === "music") continue;
        const media = await prefetchMedia(next).catch(() => undefined);
        if (!live) return;
        if (media?.videos[0]) void getLibrarySource().warm([media.videos[0]]).catch(() => undefined);
      }
    }, 300);
    return () => { live = false; clearTimeout(timer); };
  }, [index, loading, cursor]);

  // A video that plays keeps its opening bytes in memory, so opening it again starts at once.
  const playingVideo = !state.paused && media?.kind === "video" ? media.videos[0] : undefined;
  useEffect(() => { if (playingVideo) void getLibrarySource().warm([playingVideo]).catch(() => undefined); }, [playingVideo]);

  const move = useCallback((delta: number) => {
    if (moving.current) return;
    const next = index + delta;
    if (next < 0 || next >= cursor.total) return;
    moving.current = true;
    void cursor.at(next).then((value) => {
      if (!value) return;
      setDir(delta); setIndex(next); setEntry(value); setTotal(cursor.total); setElement(null);
    }).finally(() => { moving.current = false; });
  }, [index, cursor]);

  const close = useCallback(() => { pauseAll(); onClose(index); }, [index, onClose]);

  // Show the controls on any activity; hide them and the cursor after two idle seconds while playing.
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const playing = element ? !state.paused : entry?.kind === "gallery";
  const hideable = playing && !overControls && !sheet;
  const wake = useCallback(() => {
    setChrome(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setChrome(false), IDLE_MS);
  }, []);
  useEffect(() => { wake(); return () => clearTimeout(timer.current); }, [wake, index]);
  const shown = chrome || !hideable;

  useEffect(() => {
    const update = () => setFullscreen(!!fullscreenElement());
    document.addEventListener("fullscreenchange", update);
    document.addEventListener("webkitfullscreenchange", update);
    return () => { document.removeEventListener("fullscreenchange", update); document.removeEventListener("webkitfullscreenchange", update); };
  }, []);

  // Keys work wherever focus is, as long as no menu is open.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (document.querySelector('[role="menu"][data-state="open"], [data-slot="popover-content"][data-state="open"], [data-slot="dialog-content"][data-state="open"]')) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      const key = event.key;
      const handled = () => { event.preventDefault(); event.stopPropagation(); wake(); };
      if (key === "Escape") { handled(); if (sheet) setSheet(false); else if (fullscreenElement()) toggleFullscreen(root.current); else close(); return; }
      if ((event.target as HTMLElement | null)?.closest('button, input, [role="menuitem"]')) return;
      if (key === "ArrowDown" || key === "PageDown" || key === "j") { handled(); move(1); return; }
      if (key === "ArrowUp" || key === "PageUp" || key === "k") { handled(); move(-1); return; }
      if (key === "ArrowLeft" || key === "ArrowRight") {
        handled();
        const delta = key === "ArrowLeft" ? -1 : 1;
        if (gallery) gallery.move(delta);
        else if (element) element.currentTime = Math.min(Math.max(element.currentTime + delta * 5, 0), element.duration || 0);
        return;
      }
      if (key === " ") { handled(); togglePlay(element); return; }
      if (key === "m" || key === "M") { handled(); setSound({ muted: !sound.muted }); return; }
      if (key === "f" || key === "F") { handled(); toggleFullscreen(root.current); return; }
      if (key === "i" || key === "I") { handled(); if (wide) setInfo((v) => !v); else setSheet((v) => !v); }
    };
    const onKeyUp = (event: KeyboardEvent) => { if (event.key === " ") event.preventDefault(); };
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("keyup", onKeyUp, true);
    return () => { window.removeEventListener("keydown", onKey, true); window.removeEventListener("keyup", onKeyUp, true); };
  }, [move, close, gallery, element, sound.muted, wide, sheet, wake]);

  // One wheel or trackpad gesture moves one item, however long its inertia runs.
  useEffect(() => {
    const el = root.current;
    if (!el) return;
    let sum = 0, locked = false, movedAt = 0;
    let quiet: ReturnType<typeof setTimeout> | undefined;
    const onWheel = (event: WheelEvent) => {
      const area = (event.target as Element | null)?.closest?.("[data-scroll-area]");
      if (area && area.scrollHeight > area.clientHeight) return;
      event.preventDefault();
      clearTimeout(quiet);
      quiet = setTimeout(() => { locked = false; sum = 0; }, 160);
      // Inertia can pause between events; after a move, nothing counts until the slide has settled.
      if (locked || performance.now() - movedAt < 600 || Math.abs(event.deltaY) < Math.abs(event.deltaX)) return;
      sum += event.deltaY;
      if (Math.abs(sum) >= 24) { locked = true; movedAt = performance.now(); move(sum > 0 ? 1 : -1); sum = 0; }
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => { el.removeEventListener("wheel", onWheel); clearTimeout(quiet); };
  }, [move]);

  const onSlide = useCallback((n: number, count: number) => setSlideInfo({ index: n, count }), []);
  const onElement = useCallback((value: HTMLMediaElement | null) => setElement(value), []);
  const key = entry ? `${entry.kind}:${entry.id}` : "empty";
  const isGallery = entry?.kind === "gallery";
  const hasSound = !isGallery || !!media?.music.url;
  const fade = cn("transition-opacity duration-300 ease-out", shown ? "opacity-100" : "pointer-events-none opacity-0");
  const panel = entry && <InfoPanel entry={entry} media={media} />;

  return createPortal(<TooltipProvider delayDuration={500}>
    <motion.div ref={root} data-testid="viewer" role="dialog" aria-modal="true" aria-label={t("viewer.title")}
      initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ duration: 0.18, ease: "easeOut" }}
      onPointerMove={wake} onPointerDown={wake}
      className={cn("dark fixed inset-0 z-50 flex overflow-hidden bg-viewer text-viewer-foreground select-none", !shown && "cursor-none")}>
      <AnimatePresence initial={false}>
        {entry && <motion.div key={key} aria-hidden="true" className="pointer-events-none absolute inset-0" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.4 }}>
          <img src={entry.cover} alt="" className="absolute inset-0 size-full scale-125 object-cover opacity-60 blur-3xl saturate-150" />
          <div className="absolute inset-0 bg-viewer-scrim" />
        </motion.div>}
      </AnimatePresence>

      <div data-testid="viewer-stage" className="relative min-w-0 flex-1 overflow-hidden">
        <AnimatePresence initial={false} custom={dir}>
          {entry && <motion.div key={key} custom={dir} variants={slide} initial="enter" animate="center" exit="exit"
            transition={{ duration: 0.42, ease: slideEase }} className="absolute inset-0">
            {isGallery
              ? <GalleryStage entry={entry} media={media} chrome={shown} badgeSlot={badgeSlot} onElement={onElement} onHandle={setGallery} onSlide={onSlide} onExpired={refresh} />
              : <VideoStage entry={entry} media={media} onElement={onElement} onExhausted={refresh} onDoubleClick={() => toggleFullscreen(root.current)} />}
          </motion.div>}
        </AnimatePresence>

        {(loading || state.waiting) && !error && !isGallery && <div data-testid="viewer-spinner" className="spinner-late pointer-events-none absolute inset-0 grid place-items-center"><Spinner className="size-7 text-viewer-foreground/80" /></div>}
        {error && !isGallery && <div role="alert" className="absolute inset-0 grid place-items-center">
          <div className="flex flex-col items-center gap-3 rounded-xl bg-viewer-scrim px-6 py-5 text-center backdrop-blur-md">
            <p className="text-sm font-medium">{t(`viewer.errors.${error}`, { defaultValue: t("viewer.unavailable") })}</p>
            <Button size="sm" variant="secondary" onClick={retry} className="gap-1.5"><RotateCw className="size-3.5" />{t("viewer.retry")}</Button>
          </div>
        </div>}

        <div className={cn("absolute inset-x-0 top-0 flex h-16 items-center gap-2 bg-linear-to-b from-viewer-scrim to-transparent px-4", fade)}>
          <ControlButton label={t("viewer.close")} hint="Esc" side="bottom" onClick={close} className="bg-viewer-control backdrop-blur-md"><X /></ControlButton>
          <span data-testid="viewer-counter" className="ml-1 text-xs font-medium tabular-nums text-viewer-muted">{(index + 1).toLocaleString()} / {total.toLocaleString()}</span>
          <div ref={setBadgeSlot} className="flex items-center" />
          <div className="flex-1" />
          <ControlButton label={(wide ? info : sheet) ? t("viewer.hideInfo") : t("viewer.showInfo")} hint="I" side="bottom" onClick={() => wide ? setInfo(!info) : setSheet(!sheet)} className="bg-viewer-control backdrop-blur-md">
            {(wide ? info : sheet) ? <PanelRightClose /> : <PanelRightOpen />}
          </ControlButton>
        </div>

        {/* The item pair ends on the center line, so a gallery's next-image arrow sits just below it. */}
        <div className={cn("absolute right-4 bottom-1/2 flex flex-col gap-2", fade)}>
          <ControlButton data-testid="viewer-prev" label={t("viewer.previousItem")} hint="↑" side="left" onClick={() => move(-1)} disabled={index === 0} className="size-10 bg-viewer-control backdrop-blur-md disabled:opacity-30"><ChevronUp className="size-5" /></ControlButton>
          <ControlButton data-testid="viewer-next" label={t("viewer.nextItem")} hint="↓" side="left" onClick={() => move(1)} disabled={index >= total - 1} className="size-10 bg-viewer-control backdrop-blur-md disabled:opacity-30"><ChevronDown className="size-5" /></ControlButton>
        </div>

        <div data-testid="viewer-controls" onPointerEnter={() => setOverControls(true)} onPointerLeave={() => setOverControls(false)}
          className={cn("absolute inset-x-0 bottom-0 bg-linear-to-t from-viewer-scrim via-viewer-scrim/50 to-transparent px-5 pt-14 pb-3", fade)}>
          {!isGallery && <SeekBar element={element} state={state} className="mb-1" />}
          <div className="flex items-center gap-1">
            {hasSound && <ControlButton data-testid="viewer-play" label={state.paused ? t("viewer.play") : t("viewer.pause")} hint="Space" onClick={() => togglePlay(element)}>{state.paused ? <Play className="fill-current" /> : <Pause className="fill-current" />}</ControlButton>}
            {!isGallery && <TimeLabel state={state} />}
            {isGallery && media?.music.title && <span className="ml-1 flex min-w-0 items-center gap-1.5 text-xs text-viewer-muted"><Music2 className="size-3.5 shrink-0" /><span className="truncate">{media.music.title}</span></span>}
            <div className="flex-1" />
            {isGallery && slideInfo.count > 1 && <div className="mr-2 flex items-center gap-1.5" aria-label={`${slideInfo.index + 1} / ${slideInfo.count}`}>
              {slideInfo.count <= 12 && Array.from({ length: slideInfo.count }, (_, n) => <span key={n} className={cn("h-1 rounded-full transition-all duration-300", n === slideInfo.index ? "w-4 bg-viewer-foreground" : "w-1 bg-viewer-track")} />)}
              <span className="ml-1 text-xs font-medium tabular-nums text-viewer-muted">{slideInfo.index + 1} / {slideInfo.count}</span>
            </div>}
            {!isGallery && <SpeedMenu rate={state.rate} onRate={(rate) => { if (element) element.playbackRate = rate; }} />}
            {hasSound && <VolumeControl />}
            <ControlButton label={fullscreen ? t("viewer.exitFullscreen") : t("viewer.fullscreen")} hint="F" onClick={() => toggleFullscreen(root.current)}>{fullscreen ? <Minimize /> : <Maximize />}</ControlButton>
          </div>
        </div>
      </div>

      {wide && <AnimatePresence initial={false}>
        {info && <motion.aside key="info" initial={{ width: 0, opacity: 0 }} animate={{ width: 360, opacity: 1 }} exit={{ width: 0, opacity: 0 }} transition={{ duration: 0.28, ease: slideEase }}
          className="relative shrink-0 overflow-hidden border-l border-viewer-border bg-viewer-panel backdrop-blur-2xl">
          <div className="h-full w-[360px]">{panel}</div>
        </motion.aside>}
      </AnimatePresence>}
      {!wide && <Sheet open={sheet} onOpenChange={setSheet}>
        <SheetContent side="right" showCloseButton={false} className="dark w-[340px] border-viewer-border bg-viewer-panel p-0 text-viewer-foreground backdrop-blur-2xl sm:max-w-[340px]">
          <SheetTitle className="sr-only">{t("viewer.info")}</SheetTitle>
          {panel}
        </SheetContent>
      </Sheet>}
    </motion.div>
  </TooltipProvider>, document.body);
}
