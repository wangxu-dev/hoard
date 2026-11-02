import { memo, startTransition, useEffect, useLayoutEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { motion, useReducedMotion } from "motion/react";
import { useTranslation } from "react-i18next";
import { ArrowUp, Images, Music } from "lucide-react";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { EntryActionMenu } from "@/components/entry-actions";
import { EntryAccounts } from "@/components/entry-accounts";
import { LibraryNumber } from "@/components/library-number";
import { pageTransition } from "@/lib/motion";
import { getLibrarySource, type LibraryEntry, type ListKind, type Scope, type SortKey, type WorkType } from "@/lib/library";
import { queryClient, queryKeys } from "@/lib/query";
import { useMusic } from "@/lib/music";

// The list loads in pages of this size at any offset, so a jump far down only fetches what it shows.
const PAGE = 120;
// Missing pages in one stretch load together, up to this many per request.
const MAX_RUN_PAGES = 4;
// Missing pages load at most this often, for the rows shown when the timer fires, so a
// drag across the list asks for where it is rather than every row it passes.
const FETCH_DELAY = 50;
// A page that just loaded is not asked again for this long, even if some slots stay empty.
const REFETCH_AFTER = 2000;
// Cards keep about the same size at every window width; only the column count changes.
const CARD_MIN = 150;
const CARD_MAX = 184;
const MAX_COLUMNS = 8;
const GAP = 6;
// Fixed text block plus the 1px card border on both sides, so every row has a known height.
const CARD_TEXT = 62;
const CARD_BORDER = 2;
// Rendered rows reach two viewports past each edge, so a Space or Page Down
// jump lands on rows that are already laid out with covers decoded.
const OVERSCAN_VIEWPORTS = 2;
const MIN_OVERSCAN_ROWS = 8;
// Right after a leap of more than this many viewports (a jump or a timeline drag) only the
// rows in view mount, so they show at once; the overscan returns once scrolling rests.
const LEAP_VIEWPORTS = 2;
const LEAP_REST = 250;
// While the grid leaps (a timeline drag), covers that mount wait until it has held still this
// long, so rows that only flash past never queue a download ahead of the rows that stay.
const LEAP_COVER_HOLD = 80;
let coversHeldUntil = 0;
// Background decoding of rows beyond, and the overscan coming back after a leap, wait up to
// this long for the covers in view to arrive, so those load first.
const WARM_WAIT = 1000;
const coversPending = () => !!document.querySelector("[data-cover-pending], [data-placeholder]")
  || [...document.querySelectorAll<HTMLImageElement>("[data-testid=library-card] img")].some((img) => !img.complete);
// Covers this many viewports beyond the rendered rows decode in the background.
const WARM_VIEWPORTS = 2;
const PREFETCH_VIEWPORTS = 4;

function columnsFor(width: number) {
  const columns = Math.min(MAX_COLUMNS, Math.max(1, Math.floor((width + GAP) / (CARD_MIN + GAP))));
  const card = Math.min(CARD_MAX, (width - GAP * (columns - 1)) / columns);
  return { columns, card, width: columns * card + GAP * (columns - 1) };
}

// The first cover URL shown for an item stays its cover for this session, and
// a URL that finished loading once never fades in again.
const shownCover = new Map<string, string>();
const loadedCovers = new Set<string>();
// Recently shown covers stay referenced, so their decoded images are still in
// memory when a scrolled-away row mounts again.
// About 0.55 MB decoded per 323×430 cover, so the warm set stays near 260 MB at most.
const WARM_COVERS = 480;
// WebKit has no requestIdleCallback, so idle work falls back to a short timeout.
const whenIdle = (fn: () => void) => typeof window.requestIdleCallback === "function"
  ? (() => { const h = window.requestIdleCallback(fn, { timeout: 200 }); return () => window.cancelIdleCallback(h); })()
  : (() => { const h = window.setTimeout(fn, 60); return () => window.clearTimeout(h); })();
const warmCovers = new Map<string, HTMLImageElement>();
function keepWarm(src: string, decode = false) {
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
function coverOf(entry: LibraryEntry) {
  const key = `${entry.kind === "music" ? "m" : "w"}:${entry.id}`;
  const first = shownCover.get(key);
  if (first !== undefined) return first;
  shownCover.set(key, entry.cover);
  return entry.cover;
}

function preload(src: string) {
  if (!src || loadedCovers.has(src)) return Promise.resolve();
  const img = new Image();
  img.referrerPolicy = "no-referrer";
  img.src = src;
  return img.decode().then(() => { loadedCovers.add(src); }).catch(() => undefined);
}

function duration(ms: number) {
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

/** Where the grid's rows are, for the timeline beside it. */
export interface GridGeometry { columns: number; rowHeight: number; scrollMargin: number; count: number }

// Loaded lists stay in the app-wide cache, so a page that mounts again shows
// them at once; where the reader was is kept with them.
// Slots hold the loaded items at their list positions; positions not loaded yet are empty.
type Slot = LibraryEntry | undefined;
interface GridCache { items: Slot[]; total: number; shift: number }
const gridScroll = new Map<string, number>();
// The last request to show the playing sound that a grid handled, across mounts.
let handledLocate = 0;
// The sticky toolbar above and the music player below cover part of the viewport.
const TOP_INSET = 64;
const BOTTOM_INSET = 96;

/** pulseCard replays a card's hover growth once, to point at it. */
function pulseCard(element: HTMLElement) {
  element.style.zIndex = "10";
  const animation = element.animate([
    { transform: "scale(1)", easing: "ease-out" },
    { transform: "scale(1.025)", offset: 0.25, easing: "ease-out" },
    { transform: "scale(1.025)", offset: 0.7, easing: "ease-out" },
    { transform: "scale(1)" },
  ], { duration: 600 });
  animation.onfinish = animation.oncancel = () => { element.style.zIndex = ""; };
}

/** settled resolves once smooth scrolling has stopped, or after a second and a half. */
function scrollSettled(element: HTMLElement): Promise<void> {
  return new Promise((done) => {
    let last = -1, still = 0;
    const start = performance.now();
    const tick = () => {
      const top = element.scrollTop;
      still = top === last ? still + 1 : 0;
      last = top;
      if (still >= 3 || performance.now() - start > 1500) done(); else requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
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

const EntryCard = memo(function EntryCard({ entry, onOpen, fresh = false, playing }: { entry: LibraryEntry; onOpen: (entry: LibraryEntry) => void; fresh?: boolean; playing?: "playing" | "paused" }) {
  const { t } = useTranslation("library");
  const music = entry.kind === "music";
  return <EntryActionMenu entry={entry} onPlay={() => onOpen(entry)}><Card data-testid="library-card" data-entry={`${entry.kind}:${entry.id}`} tabIndex={0} onClick={() => onOpen(entry)} onKeyDown={(event) => { if (event.key === "Enter") onOpen(entry); }} className={`group group/entry relative gap-0 overflow-hidden rounded-md py-0 shadow-none transition-[transform,scale,box-shadow] duration-(--duration-slow) ease-(--ease-out) hover:z-10 hover:scale-[1.015] hover:shadow-sm motion-reduce:transition-none motion-reduce:hover:scale-100!${fresh ? " animate-in fade-in-0 zoom-in-95 duration-500 motion-reduce:animate-none" : ""}`}>
      <div className="relative">
        <Cover src={coverOf(entry)} square={music} />
        <div className="absolute top-1.5 left-1.5"><EntryAccounts entry={entry} /></div>
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
        {entry.kind === "gallery" && <span data-gallery-count className="flex shrink-0 items-center gap-1 tabular-nums" aria-label={t("grid.gallery", { count: entry.imageCount })}>
          {entry.imageCount}<Images className="size-3" aria-hidden="true" />
        </span>}
      </div>
    </CardContent>
  </Card></EntryActionMenu>;
});

function indexIds(items: Slot[]) {
  const where = new Map<string, number>();
  items.forEach((entry, i) => { if (entry) where.set(entry.id, i); });
  return where;
}

// Puts a page of the list at its positions. Items already there stay the same objects, so
// memoized cards and their images are left alone; an item shown elsewhere moves to its new place.
function place(old: Slot[], where: Map<string, number>, offset: number, incoming: LibraryEntry[]) {
  let next: Slot[] | null = null;
  incoming.forEach((entry, i) => {
    const at = offset + i;
    if (at < 0) return;
    const current = (next ?? old)[at];
    if (current?.id === entry.id) return;
    next ??= old.slice();
    const before = where.get(entry.id);
    if (before !== undefined && before !== at && next[before]?.id === entry.id) next[before] = undefined;
    if (current && where.get(current.id) === at) where.delete(current.id);
    next[at] = entry;
    where.set(entry.id, at);
  });
  return next ?? old;
}

const PlaceholderCard = memo(function PlaceholderCard({ square }: { square: boolean }) {
  return <div data-placeholder className="flex flex-col gap-1.5 overflow-hidden rounded-md border" style={{ height: "100%" }}>
    <Skeleton className={`w-full rounded-none ${square ? "aspect-square" : "aspect-[3/4]"}`} />
    <Skeleton className="mx-2 h-3 rounded-sm" />
    <Skeleton className="mx-2 h-3 w-1/2 rounded-sm" />
  </div>;
});

export const LibraryGrid = memo(function LibraryGrid({ scope, kind, type, query, sort, version, syncing, onOpen, onItemsChange, onTotalChange, onGeometry }: { scope: Scope; kind: ListKind; type: WorkType; query: string; sort: SortKey; version: number; syncing: boolean; onOpen: (entry: LibraryEntry) => void; onItemsChange: (items: (LibraryEntry | undefined)[]) => void; onGeometry?: (geometry: GridGeometry) => void; onTotalChange: (total: number) => void }) {
  const { t } = useTranslation("library");
  const filterKey = JSON.stringify([scope.accountId, scope.source, kind, type, query, sort]);
  const [cached] = useState(() => queryClient.getQueryData<GridCache>(queryKeys.grid(filterKey)));
  const [items, setItems] = useState<Slot[]>(() => cached?.items ?? []);
  // total counts the list as the library has it now; shift is how many new items at its head
  // wait behind the pill, so the grid shows total - shift positions and asks pages shifted by it.
  const [total, setTotal] = useState(() => cached?.total ?? 0);
  const [shift, setShift] = useState(() => cached?.shift ?? 0);
  const shiftRef = useRef(shift);
  shiftRef.current = shift;
  const [loading, setLoading] = useState(!cached);
  const [pending, setPending] = useState(false);
  const [settled, setSettled] = useState(!!cached);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const request = useRef(0);
  const loaded = useRef<Slot[]>(cached?.items ?? []);
  const where = useRef(indexIds(cached?.items ?? []));
  const inflight = useRef(new Set<number>());
  const fetchedAt = useRef(new Map<number, number>());
  const fetchPage = (offset: number, limit: number) => getLibrarySource().list({ scope, kind, type: kind === "music" ? "all" : type, query, sort, offset, limit });
  const apply = (next: Slot[], count: number, nextShift = shiftRef.current) => {
    loaded.current = next; shiftRef.current = nextShift;
    setItems(next); setTotal(count); setShift(nextShift); setError(false); setSettled(true);
    queryClient.setQueryData<GridCache>(queryKeys.grid(filterKey), { items: next, total: count, shift: nextShift });
  };
  // A new list, or one whose positions moved, loads its pages afresh.
  const resetPages = (next: Slot[]) => { where.current = indexIds(next); inflight.current.clear(); fetchedAt.current.clear(); };
  const lastRetry = useRef(retry);
  // A filter change keeps the current grid dimmed until the new list and its first covers are ready.
  // A list loaded before shows at once from the cache and only checks for new items.
  useEffect(() => {
    const id = ++request.current;
    const forced = lastRetry.current !== retry;
    lastRetry.current = retry;
    const known = forced ? undefined : queryClient.getQueryData<GridCache>(queryKeys.grid(filterKey));
    if (known) {
      if (loaded.current !== known.items) { resetPages(known.items); apply(known.items, known.total, known.shift); }
      setLoading(false); setPending(false);
      refreshHead(id);
      return;
    }
    setLoading(true); setPending(true);
    fetchPage(0, PAGE).then(async (page) => {
      if (id !== request.current) return;
      await Promise.race([Promise.all(page.items.slice(0, 24).map((item) => preload(coverOf(item)))), new Promise((done) => setTimeout(done, 300))]);
      if (id !== request.current) return;
      resetPages([]);
      apply(place([], where.current, 0, page.items), page.total, 0);
      fetchedAt.current.set(0, performance.now());
    }).catch(() => { if (id === request.current) { setError(true); setSettled(true); } })
      .finally(() => { if (id === request.current) { setLoading(false); setPending(false); } });
  }, [filterKey, retry]);
  // Items a sync adds ahead of the loaded ones. Near the top they slide in at once;
  // further down the grid stays put and a pill offers them, so nothing shifts under the reader.
  const [incoming, setIncoming] = useState<{ items: LibraryEntry[]; more: boolean } | null>(null);
  const fresh = useRef(new Set<string>());
  const nearTop = () => (scroller()?.scrollTop ?? 0) < 120;
  const reveal = (next: { items: LibraryEntry[]; more: boolean }) => {
    setIncoming(null);
    // More new items than one page: start the list over rather than guess the gap.
    if (next.more) { setRetry((n) => n + 1); return; }
    for (const e of next.items) fresh.current.add(e.id);
    const items = [...next.items, ...loaded.current];
    resetPages(items);
    apply(items, Math.max(items.length, totalRef.current), 0);
    setTimeout(() => { for (const e of next.items) fresh.current.delete(e.id); }, 800);
  };
  // Checks the head of the list for items a sync added since it was loaded.
  const totalRef = useRef(total);
  totalRef.current = total;
  function refreshHead(id: number) {
    fetchPage(0, PAGE).then((page) => {
      if (id !== request.current) return;
      if (!where.current.size) { resetPages([]); apply(place([], where.current, 0, page.items), page.total, 0); return; }
      const head: LibraryEntry[] = [];
      for (const e of page.items) { if (where.current.has(e.id)) break; head.push(e); }
      const more = head.length === page.items.length && page.items.length > 0;
      console.debug("[hoard:ui] grid live update", { kind, fresh: head.length, more, total: page.total });
      // Older items a backfill found extend the list at its end; their positions load when shown.
      totalRef.current = page.total;
      // The rest of the first page may now hold items the list had not loaded yet.
      apply(place(loaded.current, where.current, 0, page.items.slice(head.length)), page.total, head.length);
      if (!head.length) return;
      const next = { items: head, more };
      if (nearTop()) reveal(next); else setIncoming(next);
    }).catch(() => undefined);
  }
  // The library version counts changes app-wide; only changes after mounting need a check here.
  const mountedVersion = useRef(version);
  useEffect(() => {
    if (version === mountedVersion.current) return;
    mountedVersion.current = version;
    refreshHead(request.current);
  }, [version]);
  useEffect(() => { setIncoming(null); }, [filterKey]);
  // Scrolling back to the top by hand shows the waiting items too.
  useEffect(() => {
    const el = scroller();
    if (!incoming || !el) return;
    const onScroll = () => { if (nearTop()) reveal(incoming); };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, [incoming]);
  // State instead of a ref: the empty state unmounts the grid, and remounting must reattach the observer.
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const [layout, setLayout] = useState(() => columnsFor(1000));
  const [scrollMargin, setScrollMargin] = useState(0);
  const [measured, setMeasured] = useState(false);
  const scroller = () => host?.closest<HTMLElement>("[data-testid=main-scroll]") ?? null;
  useLayoutEffect(() => {
    const element = host;
    if (!element) return;
    const measure = () => {
      // A window not shown yet measures 0 wide; keep the last layout until a real width arrives.
      if (!element.clientWidth) return;
      setLayout((old) => { const next = columnsFor(element.clientWidth); return old.columns === next.columns && Math.abs(old.card - next.card) < 0.5 ? old : next; });
      const parent = scroller();
      setMeasured(true);
      if (parent) setScrollMargin(element.getBoundingClientRect().top - parent.getBoundingClientRect().top + parent.scrollTop);
    };
    measure();
    // Window resizes remeasure once a frame; only the column count and centering change.
    let frame = 0;
    const observer = new ResizeObserver(() => { cancelAnimationFrame(frame); frame = requestAnimationFrame(measure); });
    observer.observe(element);
    return () => { observer.disconnect(); cancelAnimationFrame(frame); };
  }, [settled, host]);
  // Rows cover the whole known total; positions not loaded yet draw same-size placeholders.
  const square = kind === "music";
  const count = Math.max(0, total - shift);
  const cardHeight = (square ? layout.card : layout.card * 4 / 3) + CARD_TEXT + CARD_BORDER;
  const rowHeight = cardHeight + GAP;
  const viewportRows = Math.ceil((scroller()?.clientHeight ?? window.innerHeight) / rowHeight);
  // Before the list answers, placeholders fill the window.
  const rows = settled ? Math.ceil(count / layout.columns) : viewportRows;
  const [leaping, setLeaping] = useState(false);
  useEffect(() => {
    const el = scroller();
    if (!el) return;
    let last = el.scrollTop, timer = 0;
    const onScroll = () => {
      const top = el.scrollTop;
      if (Math.abs(top - last) > el.clientHeight * LEAP_VIEWPORTS) {
        coversHeldUntil = performance.now() + LEAP_COVER_HOLD;
        setLeaping(true);
        window.clearTimeout(timer);
        const rest = performance.now() + LEAP_REST;
        const settle = () => {
          if (coversPending() && performance.now() - rest < WARM_WAIT) timer = window.setTimeout(settle, 50);
          else startTransition(() => setLeaping(false));
        };
        timer = window.setTimeout(settle, LEAP_REST);
      }
      last = top;
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => { el.removeEventListener("scroll", onScroll); window.clearTimeout(timer); };
  }, [settled]);
  const fullOverscan = Math.max(MIN_OVERSCAN_ROWS, viewportRows * OVERSCAN_VIEWPORTS + 1);
  const overscan = leaping ? 1 : fullOverscan;
  const virtualizer = useVirtualizer({ count: rows, getScrollElement: scroller, estimateSize: () => rowHeight, overscan, scrollMargin });
  useEffect(() => { virtualizer.measure(); }, [rowHeight]);
  // Returning to the page puts the reader back where they were in this list.
  const restoreScroll = useRef(!!cached);
  useLayoutEffect(() => {
    const el = scroller();
    const top = restoreScroll.current ? gridScroll.get(filterKey) : undefined;
    restoreScroll.current = false;
    if (el && top) el.scrollTop = top;
    return () => { const at = scroller(); if (at) gridScroll.set(filterKey, at.scrollTop); };
  }, [filterKey]);
  const music = useMusic();
  const playingId = kind === "music" ? music.playing?.id : undefined;
  const playingState = music.playing?.paused ? "paused" : "playing";
  // Where the playing sound sits in this list. The player's own position counts when it
  // plays this very list, so a sound beyond the loaded pages can be reached too.
  const reduced = useReducedMotion();
  const queued = music.queue?.cursor.query;
  const sameList = !!queued && kind === "music" && queued.kind === "music" && queued.query === query && queued.sort === sort
    && (queued.scope.accountId ?? "") === (scope.accountId ?? "") && (queued.scope.source ?? "") === (scope.source ?? "");
  const playingIndex = !playingId ? -1 : sameList ? music.queue!.index : items.findIndex((e) => e?.id === playingId);
  /** cellVisible reports whether a cell is wholly in view, or with partly any of it. */
  const cellVisible = (index: number, partly = false) => {
    const el = scroller();
    if (!el || index < 0) return false;
    const top = Math.floor(index / layout.columns) * rowHeight + scrollMargin;
    const viewTop = el.scrollTop + TOP_INSET, viewBottom = el.scrollTop + el.clientHeight - BOTTOM_INSET;
    return partly ? top + cardHeight > viewTop && top < viewBottom : top >= viewTop && top + cardHeight <= viewBottom;
  };
  const showCell = (index: number, id: string, pulse: boolean) => {
    const el = scroller();
    if (!el || index < 0) return;
    if (!cellVisible(index)) {
      const top = Math.floor(index / layout.columns) * rowHeight + scrollMargin;
      el.scrollTo({ top: Math.max(0, top - (el.clientHeight - cardHeight) / 2), behavior: reduced ? "instant" : "smooth" });
    }
    if (!pulse || reduced) return;
    // The cell may still be a placeholder while its page loads; the pulse waits for the card.
    void scrollSettled(el).then(async () => {
      for (let tries = 0; tries < 20; tries++) {
        const card = document.querySelector<HTMLElement>(`[data-entry="music:${CSS.escape(id)}"]`);
        if (card) { pulseCard(card); return; }
        await new Promise((done) => setTimeout(done, 50));
      }
    });
  };
  // Follow: when the sound changes while the reader can see the previous one, the new one is
  // brought into view; after the reader scrolled away the grid stays where it is.
  const previous = useRef<{ id: string; index: number } | null>(null);
  useEffect(() => {
    const before = previous.current;
    if (playingId && playingIndex >= 0) previous.current = { id: playingId, index: playingIndex };
    if (!before || !playingId || before.id === playingId || playingIndex < 0) return;
    if (cellVisible(before.index, true) && !cellVisible(playingIndex)) showCell(playingIndex, playingId, false);
  }, [playingId, playingIndex]);
  // Locate: the music player asked to show the playing sound.
  useEffect(() => {
    if (music.locate <= handledLocate || !measured || !settled || pending || kind !== "music" || !playingId || playingIndex < 0) return;
    handledLocate = music.locate;
    showCell(playingIndex, playingId, true);
  }, [music.locate, measured, settled, pending, kind, playingId, playingIndex, layout.columns, scrollMargin]);
  const visible = virtualizer.getVirtualItems();
  const firstRow = visible.length ? visible[0].index : 0;
  const lastRow = visible.length ? visible[visible.length - 1].index : 0;
  // Decodes covers of the rows just beyond the rendered ones, above and below,
  // so even a jump past the overscan finds them ready in memory.
  // Covers on screen download first: the cover cache serves the newest request first, so
  // warming rows beyond waits until those are in (or a second has passed).
  useEffect(() => {
    if (!settled || leaping) return;
    const span = viewportRows * WARM_VIEWPORTS * layout.columns;
    const below = items.slice((lastRow + 1) * layout.columns, (lastRow + 1) * layout.columns + span);
    const above = items.slice(Math.max(0, firstRow * layout.columns - span), firstRow * layout.columns);
    const start = performance.now();
    let timer = 0, cancel = () => window.clearTimeout(timer);
    const attempt = () => {
      if (coversPending() && performance.now() - start < WARM_WAIT) { timer = window.setTimeout(attempt, 100); return; }
      cancel = whenIdle(() => { for (const entry of [...below, ...above]) if (entry) keepWarm(coverOf(entry), true); });
    };
    attempt();
    return () => cancel();
  }, [firstRow, lastRow, items, layout.columns, viewportRows, settled, leaping]);
  // Loads the pages the rendered rows need, then the ones a few viewports below. Missing
  // pages in one stretch come in one request, so a jump costs one or two.
  const loadShown = useRef<() => void>(() => undefined);
  const retryTimer = useRef(0);
  loadShown.current = () => {
    if (!settled || pending || error || !count) return;
    const id = request.current;
    const load = (from: number, to: number) => {
      to = Math.min(to, count);
      const now = performance.now();
      const missing: number[] = [];
      for (let p = Math.floor(from / PAGE); p * PAGE < to; p++) {
        if (inflight.current.has(p) || now - (fetchedAt.current.get(p) ?? -Infinity) < REFETCH_AFTER) continue;
        const end = Math.min((p + 1) * PAGE, count);
        for (let i = p * PAGE; i < end; i++) if (!loaded.current[i]) { missing.push(p); break; }
      }
      const runs: number[][] = [];
      for (const p of missing) {
        const run = runs[runs.length - 1];
        if (run && run[run.length - 1] === p - 1 && run.length < MAX_RUN_PAGES) run.push(p); else runs.push([p]);
      }
      for (const run of runs) {
        const offset = run[0] * PAGE, asked = shiftRef.current;
        for (const p of run) inflight.current.add(p);
        fetchPage(offset + asked, run.length * PAGE).then((page) => {
          if (id !== request.current) return;
          // Positions are placed against the head as it is now, should new items have arrived meanwhile.
          const at = offset + asked - shiftRef.current;
          startTransition(() => apply(place(loaded.current, where.current, at, page.items), page.total));
        }).catch(() => {
          // A failed page is asked again after REFETCH_AFTER; the grid keeps loading the rest.
          if (id !== request.current) return;
          console.warn("[hoard:ui] grid page failed", { from: run[0], pages: run.length });
          window.clearTimeout(retryTimer.current);
          retryTimer.current = window.setTimeout(() => loadShown.current(), REFETCH_AFTER + 50);
        }).finally(() => {
          if (id !== request.current) return;
          const done = performance.now();
          for (const p of run) { inflight.current.delete(p); fetchedAt.current.set(p, done); }
        });
      }
    };
    // Pages are chosen from the rows in view with the full overscan, even while a leap
    // mounts fewer, so the overscan coming back needs nothing more.
    const columns = layout.columns;
    const el = scroller();
    const top = Math.max(0, (el?.scrollTop ?? 0) - scrollMargin);
    const shownFirst = Math.floor(top / rowHeight), shownLast = Math.floor((top + (el?.clientHeight ?? 800)) / rowHeight);
    const from = Math.max(0, shownFirst - fullOverscan), to = shownLast + 1 + fullOverscan;
    load(from * columns, to * columns);
    load(to * columns, (to + viewportRows * PREFETCH_VIEWPORTS) * columns);
  };
  const loadTimer = useRef(0);
  useEffect(() => {
    if (loadTimer.current) return;
    loadTimer.current = window.setTimeout(() => { loadTimer.current = 0; loadShown.current(); }, FETCH_DELAY);
  }, [firstRow, lastRow, count, items, settled, pending, error, filterKey, layout.columns, viewportRows]);
  // Cleared timers are forgotten too: StrictMode mounts twice in development, and a stale id would block every later load.
  useEffect(() => () => { window.clearTimeout(loadTimer.current); loadTimer.current = 0; window.clearTimeout(retryTimer.current); }, []);
  useEffect(() => { if (settled && measured) onGeometry?.({ columns: layout.columns, rowHeight, scrollMargin, count }); }, [settled, measured, layout.columns, rowHeight, scrollMargin, count, onGeometry]);
  useEffect(() => { onItemsChange(items); onTotalChange(total); }, [items, total, onItemsChange, onTotalChange]);
  if (error && !where.current.size) return <div role="alert" className="flex items-center gap-3 py-10 text-sm text-muted-foreground">{t("grid.loadFailed")}<Button variant="outline" size="sm" onClick={() => { setError(false); setRetry((n) => n + 1); }}>{t("grid.retry")}</Button></div>;
  if (settled && !pending && count === 0) return <div data-testid="library-empty" className="flex min-h-[calc(100dvh-8rem)] items-center justify-center gap-2 text-center text-sm text-muted-foreground">
    {syncing && <Spinner className="size-4" />}{query || type !== "all" ? t("grid.emptyFiltered") : syncing ? t("grid.emptySyncing") : t("grid.empty")}
  </div>;
  return <motion.div className="flex min-w-0 flex-col gap-2" initial={false} animate={{ opacity: pending && count > 0 ? 0.55 : 1 }} transition={pageTransition}>
    <div ref={setHost} className="min-w-0">
      <div className="mx-auto flex flex-col gap-2" style={{ width: layout.width }}>
        {incoming && <div className="pointer-events-none sticky top-16 z-20 flex h-0 justify-center">
          <Button data-testid="grid-incoming" size="sm" variant="secondary" onClick={() => { scroller()?.scrollTo({ top: 0, behavior: "smooth" }); }}
            className="pointer-events-auto mt-2 h-8 gap-1.5 rounded-full px-3.5 shadow-md animate-in fade-in-0 slide-in-from-top-2 duration-300">
            <ArrowUp className="size-3.5" />{t("grid.incoming", { count: incoming.items.length, more: incoming.more ? "+" : "" })}
          </Button>
        </div>}
        <p className="text-xs text-muted-foreground tabular-nums">{t("grid.totalPrefix")}<LibraryNumber value={total} />{t("grid.totalSuffix")}</p>
        <div data-testid="library-grid" className="relative" style={{ height: virtualizer.getTotalSize() }}>
          {visible.map((row) => {
            const from = row.index * layout.columns;
            const cells = Math.min(layout.columns, (settled ? count : rows * layout.columns) - from);
            return <div key={row.key} className="absolute top-0 left-0 grid w-full"
              style={{ height: cardHeight, transform: `translateY(${row.start - scrollMargin}px)`, gridTemplateColumns: `repeat(${layout.columns}, ${layout.card}px)`, gap: GAP }}>
              {Array.from({ length: cells }, (_, i) => {
                const entry = settled ? items[from + i] : undefined;
                return entry ? <EntryCard key={`${entry.kind === "music" ? "m" : "w"}:${entry.id}`} entry={entry} onOpen={onOpen} fresh={fresh.current.has(entry.id)} playing={entry.id === playingId ? playingState : undefined} /> : <PlaceholderCard key={`p:${from + i}`} square={square} />;
              })}
            </div>;
          })}
        </div>
        <div className="h-6" />
      </div>
    </div>
  </motion.div>;
});
