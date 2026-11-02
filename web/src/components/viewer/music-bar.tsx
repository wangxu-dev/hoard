import { useEffect, useRef, useState } from "react";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import { Pause, Play, Repeat, Repeat1, Shuffle, SkipBack, SkipForward, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { LibraryEntry } from "@/lib/library";
import { claimPlayback, formatClock, releaseMedia, togglePlay, useApplySound, useMediaState } from "@/lib/player";
import { canGoBack, closeMusic, cyclePlayMode, locateNowPlaying, nextIndex, playNext, playPrevious, setNowPlaying, useMusic } from "@/lib/music";
import { ControlButton, SeekBar, VolumeControl } from "./controls";
import { useMedia } from "./use-media";

// A sound that cannot play even with a fresh URL is skipped after a short notice.
const SKIP_MS = 1500;

/** MusicBar is the app-wide music player; it keeps playing while pages and tabs change. */
export function MusicBar({ className }: { className?: string }) {
  const { t } = useTranslation("library");
  const music = useMusic();
  // While the bar slides out after closing, it keeps showing the last sound.
  const last = useRef(music.queue);
  if (music.queue) last.current = music.queue;
  const { cursor, index } = last.current!;
  const [entry, setEntry] = useState<LibraryEntry | undefined>(() => cursor.peek(index));
  const [audio, setAudio] = useState<HTMLAudioElement | null>(null);
  const { media, error, refresh } = useMedia(entry?.kind === "music" ? entry : undefined);
  const state = useMediaState(audio);
  useApplySound(audio);
  useEffect(() => {
    let live = true;
    setEntry(cursor.peek(index));
    void cursor.at(index).then((value) => { if (live) setEntry(value); });
    return () => { live = false; };
  }, [cursor, index]);
  useEffect(() => () => releaseMedia(audio), [audio]);
  useEffect(() => { setNowPlaying(entry ? { id: entry.id, paused: state.paused } : null); }, [entry?.id, state.paused]);
  useEffect(() => () => setNowPlaying(null), []);
  // ← and → seek five seconds while nothing else takes the keys.
  useEffect(() => {
    if (!audio) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      const target = event.target as HTMLElement | null;
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || target?.closest("input, textarea, [contenteditable], [role=slider], [role=menu]")) return;
      event.preventDefault();
      audio.currentTime = Math.min(Math.max(audio.currentTime + (event.key === "ArrowLeft" ? -5 : 5), 0), audio.duration || 0);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [audio]);
  const src = media?.music.url;
  useEffect(() => { if (audio && src) { claimPlayback(audio); void audio.play().catch(() => undefined); } }, [audio, src]);
  useEffect(() => {
    if (!error || nextIndex(music) < 0) return;
    const timer = setTimeout(playNext, SKIP_MS);
    return () => clearTimeout(timer);
  }, [error]);
  const title = media?.music.title || entry?.title || "";
  const author = media?.music.author || entry?.author || "";
  const ModeIcon = music.mode === "one" ? Repeat1 : music.mode === "shuffle" ? Shuffle : Repeat;
  return <TooltipProvider delayDuration={500}>
    <motion.div data-testid="music-bar" initial={{ y: 24, opacity: 0 }} animate={{ y: 0, opacity: 1 }} exit={{ y: 24, opacity: 0 }} transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
      className={cn("dark @container flex w-full max-w-4xl min-w-0 items-center gap-2 rounded-2xl border border-viewer-border bg-viewer-bar px-2.5 py-2 text-viewer-foreground shadow-lg backdrop-blur-(--glass-blur) backdrop-saturate-150 @2xl:gap-3", className)}>
      {/* The cover and title show the playing sound in the library. */}
      <button type="button" data-testid="music-locate" onClick={locateNowPlaying} title={t("viewer.locate")} aria-label={`${t("viewer.locate")}: ${title}`}
        className="-my-1 -ml-1 flex min-w-0 flex-[0_1_14rem] cursor-pointer items-center gap-2 rounded-xl p-1 text-left transition-colors duration-150 hover:bg-viewer-control-hover focus-visible:ring-2 focus-visible:ring-viewer-foreground/40 focus-visible:outline-none @2xl:gap-3">
        <span className="relative size-11 shrink-0 overflow-hidden rounded-lg bg-viewer-control">
          {entry?.cover && <img src={entry.cover} alt="" className="size-full object-cover" />}
        </span>
        <span className="min-w-0">
          <span data-testid="music-title" className="block truncate text-sm font-medium">{title}</span>
          <span className={cn("block truncate text-xs", error ? "text-destructive" : "text-viewer-muted")}>{error ? t("viewer.unavailable") : author}</span>
        </span>
      </button>
      <div className="flex shrink-0 items-center">
        <ControlButton data-testid="music-previous" label={t("viewer.previous")} onClick={playPrevious} disabled={!canGoBack(music)}><SkipBack className="fill-current" /></ControlButton>
        <ControlButton data-testid="music-play" label={state.paused ? t("viewer.play") : t("viewer.pause")} onClick={() => togglePlay(audio)} className="size-10">
          {state.paused ? <Play className="size-5 translate-x-px fill-current" /> : <Pause className="size-5 fill-current" />}
        </ControlButton>
        <ControlButton data-testid="music-next" label={t("viewer.next")} onClick={playNext} disabled={nextIndex(music) < 0}><SkipForward className="fill-current" /></ControlButton>
        <ControlButton data-testid="music-mode" data-mode={music.mode} label={t(`viewer.playMode.${music.mode}`)} onClick={cyclePlayMode} className="hidden shrink-0 @xl:inline-flex"><ModeIcon /></ControlButton>
      </div>
      <span className="hidden w-10 shrink-0 text-right text-[11px] tabular-nums text-viewer-muted @2xl:inline-block">{formatClock(state.time)}</span>
      <SeekBar element={audio} state={state} className="min-w-12 flex-1" />
      <span className="hidden w-10 shrink-0 text-[11px] tabular-nums text-viewer-muted @2xl:inline-block">{formatClock(state.duration)}</span>
      <VolumeControl />
      <ControlButton data-testid="music-close" label={t("viewer.closePlayer")} onClick={closeMusic} className="shrink-0"><X /></ControlButton>
      {src && <audio ref={setAudio} src={src} preload="auto" loop={music.mode === "one"} onEnded={playNext} onError={() => refresh()} />}
    </motion.div>
  </TooltipProvider>;
}
