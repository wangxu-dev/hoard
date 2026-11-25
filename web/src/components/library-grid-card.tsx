import { memo, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Images, Music } from "lucide-react";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EntryActionMenu } from "@/components/entry-actions";
import { EntryAccounts } from "@/components/entry-accounts";
import type { LibraryEntry } from "@/lib/library";
import { prefetchMedia } from "@/lib/query";

// Fixed text block plus the 1px card border on both sides, so every row has a known height.
export const CARD_TEXT = 62;
export const CARD_BORDER = 2;

let coversHeldUntil = 0;

export function holdCoversUntil(until: number) {
  coversHeldUntil = until;
}

export const coversPending = () => !!document.querySelector("[data-cover-pending], [data-placeholder]")
  || [...document.querySelectorAll<HTMLImageElement>("[data-testid=library-card] img")].some((img) => !img.complete);

// The first cover URL shown for an item stays its cover for this session, and
// a URL that finished loading once never fades in again.
const shownCover = new Map<string, string>();
const loadedCovers = new Set<string>();
// Recently shown covers stay referenced, so their decoded images are still in
// memory when a scrolled-away row mounts again.
// About 0.55 MB decoded per 323×430 cover, so the warm set stays near 260 MB at most.
const WARM_COVERS = 480;
const warmCovers = new Map<string, HTMLImageElement>();

// WebKit has no requestIdleCallback, so idle work falls back to a short timeout.
export const whenIdle = (fn: () => void) => typeof window.requestIdleCallback === "function"
  ? (() => { const h = window.requestIdleCallback(fn, { timeout: 200 }); return () => window.cancelIdleCallback(h); })()
  : (() => { const h = window.setTimeout(fn, 60); return () => window.clearTimeout(h); })();

export function keepWarm(src: string, decode = false) {
  if (!src) return;
  const known = warmCovers.get(src);
  warmCovers.delete(src);
  const img = known ?? new Image();
  if (!known) {
    img.src = src;
    if (decode) img.decode().then(() => { loadedCovers.add(src); }).catch(() => undefined);
  }
  warmCovers.set(src, img);
  if (warmCovers.size > WARM_COVERS) warmCovers.delete(warmCovers.keys().next().value!);
}

export function coverOf(entry: LibraryEntry) {
  const key = `${entry.kind === "music" ? "m" : "w"}:${entry.id}`;
  const first = shownCover.get(key);
  if (first !== undefined) return first;
  shownCover.set(key, entry.cover);
  return entry.cover;
}

function duration(ms: number) {
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

/** NowPlaying marks the cover of the sound that plays, with bars that stop while it is paused. */
function NowPlaying({ paused }: { paused: boolean }) {
  const { t } = useTranslation("library");
  return <div data-testid="now-playing" data-paused={paused || undefined} role="img" aria-label={paused ? t("grid.paused") : t("grid.playing")}
    className="dark pointer-events-none absolute bottom-2 left-2 flex h-7 items-end gap-[3px] rounded-lg border border-viewer-border bg-viewer-bar px-2 pt-1.5 pb-[7px] text-viewer-foreground shadow-sm backdrop-blur-(--glass-blur) backdrop-saturate-150">
    {[0, 1, 2, 3].map((n) => <span key={n} className="eq-bar h-3 w-[3px] rounded-full bg-current" style={{ animationDelay: `${-n * 0.23}s`, animationDuration: `${0.8 + (n % 2) * 0.25}s` }} />)}
  </div>;
}

// Only rendered rows exist, overscan included, so every cover in the DOM is within a few
// viewports and loads eagerly instead of waiting to be scrolled near.
function Cover({ src, square }: { src: string; square: boolean }) {
  const [failed, setFailed] = useState(false);
  const [loaded, setLoaded] = useState(() => loadedCovers.has(src));
  const [requested, setRequested] = useState(() => loadedCovers.has(src) || performance.now() >= coversHeldUntil);
  useEffect(() => {
    if (requested) return;
    let timer = 0;
    const check = () => { const wait = coversHeldUntil - performance.now(); if (wait <= 0) setRequested(true); else timer = window.setTimeout(check, wait); };
    check();
    return () => window.clearTimeout(timer);
  }, [requested]);
  return <div className={`relative w-full overflow-hidden bg-muted ${square ? "aspect-square" : "aspect-[3/4]"}`}>
    {!requested ? <div data-cover-pending className="size-full" /> : src && !failed ? <img src={src} alt="" loading="eager" decoding="async" referrerPolicy="no-referrer"
      onLoad={() => { loadedCovers.add(src); keepWarm(src); setLoaded(true); }} onError={() => setFailed(true)}
      className={`size-full object-cover transition-[opacity,filter] group-hover:brightness-105 motion-reduce:transition-none ${loaded ? "opacity-100 duration-150" : "opacity-0 duration-200"}`} />
      : <div className="flex size-full items-center justify-center text-muted-foreground">{square ? <Music className="size-6" /> : <Images className="size-6" />}</div>}
  </div>;
}

export const EntryCard = memo(function EntryCard({ entry, onOpen, fresh = false, playing, readOnlyPreview = false }: { entry: LibraryEntry; onOpen: (entry: LibraryEntry) => void; fresh?: boolean; playing?: "playing" | "paused"; readOnlyPreview?: boolean }) {
  const { t } = useTranslation("library");
  const music = entry.kind === "music";
  const prefetchTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const cancelPrefetch = () => clearTimeout(prefetchTimer.current);
  const schedulePrefetch = () => {
    if (readOnlyPreview) return;
    cancelPrefetch();
    const delay = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--duration-slow"));
    prefetchTimer.current = setTimeout(() => { void prefetchMedia(entry).catch(() => undefined); }, delay);
  };
  useEffect(() => cancelPrefetch, [entry.kind, entry.id]);
  const card = <Card onPointerEnter={schedulePrefetch} onPointerLeave={cancelPrefetch} onFocus={schedulePrefetch} onBlur={cancelPrefetch} data-testid="library-card" data-entry={`${entry.kind}:${entry.id}`} tabIndex={0} onClick={() => onOpen(entry)} onKeyDown={(event) => { if (event.key === "Enter") onOpen(entry); }} className={`group group/entry relative gap-0 overflow-hidden rounded-md py-0 shadow-none transition-[transform,scale,box-shadow] duration-(--duration-slow) ease-(--ease-out) hover:z-10 hover:scale-[1.015] hover:shadow-sm motion-reduce:transition-none motion-reduce:hover:scale-100!${fresh ? " animate-in fade-in-0 zoom-in-95 duration-500 motion-reduce:animate-none" : ""}`}>
      <div className="relative">
        <Cover src={readOnlyPreview ? entry.cover : coverOf(entry)} square={music} />
        {!readOnlyPreview && <div className="absolute top-1.5 left-1.5"><EntryAccounts entry={entry} /></div>}
        {playing && <NowPlaying paused={playing === "paused"} />}
        {entry.kind !== "gallery" && entry.durationMs > 0 ? <Badge variant="secondary" className="absolute right-1.5 bottom-1.5 tabular-nums">{duration(entry.durationMs)}</Badge> : null}
      </div>
    <CardContent className="flex h-[62px] min-w-0 flex-col gap-0.5 px-2 py-1.5">
      <p className="line-clamp-2 min-h-8 text-xs leading-4">{entry.title.trim() || t("grid.untitled")}</p>
      <div className="flex min-w-0 items-center gap-1 text-[11px] leading-4 text-muted-foreground">
        {!music && <Avatar className="size-4 shrink-0 after:hidden">
          <AvatarImage src={entry.authorAvatar} alt="" />
          <AvatarFallback className="text-[10px]">{entry.author.slice(0, 1)}</AvatarFallback>
        </Avatar>}
        <span className="min-w-0 flex-1 truncate">{entry.author}</span>
        {entry.kind === "gallery" && (!readOnlyPreview || entry.imageCount > 0) && <span data-gallery-count className="flex shrink-0 items-center gap-1 tabular-nums" aria-label={t("grid.gallery", { count: entry.imageCount })}>
          {entry.imageCount}<Images className="size-3" aria-hidden="true" />
        </span>}
      </div>
    </CardContent>
  </Card>;
  return readOnlyPreview ? card : <EntryActionMenu entry={entry} onPlay={() => onOpen(entry)}>{card}</EntryActionMenu>;
});

export const PlaceholderCard = memo(function PlaceholderCard({ square }: { square: boolean }) {
  return <div data-placeholder className="flex flex-col gap-1.5 overflow-hidden rounded-md border" style={{ height: "100%" }}>
    <Skeleton className={`w-full rounded-none ${square ? "aspect-square" : "aspect-[3/4]"}`} />
    <Skeleton className="mx-2 h-3 rounded-sm" />
    <Skeleton className="mx-2 h-3 w-1/2 rounded-sm" />
  </div>;
});
