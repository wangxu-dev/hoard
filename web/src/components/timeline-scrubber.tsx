import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import type { GridGeometry } from "@/components/library-grid";
import type { TimelineMonth } from "@/lib/library";
import { cn } from "@/lib/utils";

// The track leaves room above for the hover label of the newest month and a little below.
const PAD_TOP = 32;
const PAD_BOTTOM = 28;
// Year labels and month dots thin out where months are packed together.
const MIN_YEAR_GAP = 16;
const MIN_DOT_GAP = 8;
const MIN_DOT_SEGMENT = 5;
// A year label is 14px tall and ends 2px above its month; dots keep 4px from it.
const YEAR_HEIGHT = 16;
const DOT_CLEARANCE = 4;
const INDICATOR = 32;
// The month label that follows scrolling stays this long after scrolling stops.
const LABEL_LINGER = 800;
// Month labels keep this far from the rail's ends so the window edge never cuts them.
const LABEL_HEIGHT = 28;
const LABEL_MARGIN = 8;
const labelCenter = (y: number, height: number) => Math.min(height - LABEL_MARGIN - LABEL_HEIGHT / 2, Math.max(LABEL_MARGIN + LABEL_HEIGHT / 2, y));
// A click this close below a month's top lands on its first row.
const SNAP = 4;

/** A month's place in the grid (top and bottom in grid pixels) and on the track (y and h). */
interface Segment { month: TimelineMonth; top: number; bottom: number; y: number; h: number; year: boolean; dot: boolean }

const scroller = () => document.querySelector<HTMLElement>("[data-testid=main-scroll]");
const toolbar = () => document.querySelector<HTMLElement>("[data-testid=library-toolbar]");

/** TimelineScrubber is the month rail beside the grid: it shows where the reader is and jumps anywhere. */
export function TimelineScrubber({ months, geometry, active }: { months: TimelineMonth[]; geometry: GridGeometry | null; active: boolean }) {
  const { t, i18n } = useTranslation("library");
  const root = useRef<HTMLDivElement>(null);
  const indicator = useRef<HTMLDivElement>(null);
  // The rail sits below the sticky toolbar and fills the rest of the scroll viewport.
  const [frame, setFrame] = useState({ inset: 0, height: 0 });
  useEffect(() => {
    const el = scroller(), bar = toolbar();
    if (!el) return;
    const measure = () => setFrame((old) => {
      const next = { inset: bar?.offsetHeight ?? 0, height: el.clientHeight - (bar?.offsetHeight ?? 0) };
      return old.inset === next.inset && old.height === next.height ? old : next;
    });
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    if (bar) observer.observe(bar);
    return () => observer.disconnect();
  }, []);
  const track = Math.max(0, frame.height - PAD_TOP - PAD_BOTTOM);
  const gridHeight = geometry ? Math.ceil(geometry.count / geometry.columns) * geometry.rowHeight : 0;
  const segments = useMemo<Segment[]>(() => {
    if (!geometry || !gridHeight || !track) return [];
    // New items the grid has not revealed yet are counted here but not shown there.
    const hidden = Math.max(0, months.reduce((sum, m) => sum + m.count, 0) - geometry.count);
    const rowTop = (offset: number) => Math.min(gridHeight, Math.floor(Math.max(0, offset - hidden) / geometry.columns) * geometry.rowHeight);
    let lastYear = -Infinity, lastDot = -Infinity;
    const placed = months.map((month, i) => {
      const top = rowTop(month.start);
      const bottom = i + 1 < months.length ? rowTop(months[i + 1].start) : gridHeight;
      const y = PAD_TOP + top / gridHeight * track, h = (bottom - top) / gridHeight * track;
      const year = i === 0 || (month.year !== months[i - 1].year && y - lastYear >= MIN_YEAR_GAP);
      if (year) lastYear = y;
      return { month, top, bottom, y, h, year, dot: false };
    });
    // Dots stay clear of the year labels, which sit just above their month's top.
    const labels = placed.filter((seg) => seg.year).map((seg) => seg.y);
    for (const seg of placed) {
      const clear = labels.every((y) => seg.y < y - YEAR_HEIGHT - DOT_CLEARANCE || seg.y > y + DOT_CLEARANCE);
      seg.dot = clear && seg.h > MIN_DOT_SEGMENT && seg.y - lastDot >= MIN_DOT_GAP;
      if (seg.dot) lastDot = seg.y;
    }
    return placed;
  }, [months, geometry, gridHeight, track]);
  const formats = useMemo(() => ({
    month: new Intl.DateTimeFormat(i18n.language, { year: "numeric", month: "long" }),
    year: new Intl.DateTimeFormat(i18n.language, { year: "numeric" }),
    count: new Intl.NumberFormat(i18n.language),
  }), [i18n.language]);
  const monthName = (m: TimelineMonth) => m.year ? formats.month.format(new Date(m.year, m.month - 1, 1)) : t("timeline.undated");
  const yearName = (m: TimelineMonth) => m.year ? formats.year.format(new Date(m.year, 0, 1)) : t("timeline.undated");
  const label = (m: TimelineMonth) => `${monthName(m)} · ${formats.count.format(m.count)}`;
  // Grid pixels of the first row below the toolbar, and the month that holds them.
  const viewTop = () => { const el = scroller(); return el && geometry ? el.scrollTop + frame.inset - geometry.scrollMargin : 0; };
  const segmentAt = (gridY: number) => {
    let found = 0;
    for (let i = 0; i < segments.length && segments[i].top <= gridY; i++) found = i;
    return found;
  };
  const scrollToGrid = (gridY: number) => {
    const el = scroller();
    if (el && geometry) el.scrollTop = geometry.scrollMargin + gridY - frame.inset;
  };
  // The indicator and the scrolling label follow the grid every frame without re-rendering.
  const [current, setCurrent] = useState<number | null>(null);
  const [scrolling, setScrolling] = useState(false);
  const indicatorY = () => PAD_TOP + Math.min(1, Math.max(0, viewTop() / (gridHeight || 1))) * track;
  useEffect(() => {
    const el = scroller();
    if (!el || !segments.length) return;
    let frameId = 0, linger = 0, first = true;
    const place = () => {
      frameId = 0;
      if (indicator.current) indicator.current.style.transform = `translateY(${indicatorY() - INDICATOR / 2}px)`;
      setCurrent(segmentAt(Math.max(0, viewTop())));
    };
    const onScroll = () => {
      if (!frameId) frameId = requestAnimationFrame(place);
      if (first) { first = false; return; }
      setScrolling(true);
      window.clearTimeout(linger);
      linger = window.setTimeout(() => setScrolling(false), LABEL_LINGER);
    };
    place();
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => { el.removeEventListener("scroll", onScroll); cancelAnimationFrame(frameId); window.clearTimeout(linger); };
  }, [segments, frame.inset, geometry?.scrollMargin]);
  // Hover shows a line and the month under the pointer; pressing jumps there and dragging follows.
  const [hover, setHover] = useState<number | null>(null);
  // The hover line and label fade out where they were, with their text, instead of jumping away empty.
  const lastHover = useRef(0);
  if (hover !== null) lastHover.current = hover;
  const hoverAt = hover ?? lastHover.current;
  const [dragging, setDragging] = useState(false);
  const pending = useRef<number | null>(null);
  const dragFrame = useRef(0);
  const trackY = (event: PointerEvent) => event.clientY - (root.current?.getBoundingClientRect().top ?? 0);
  const jump = (y: number) => {
    if (!segments.length) return;
    const clamped = Math.min(PAD_TOP + track, Math.max(PAD_TOP, y));
    let gridY = (clamped - PAD_TOP) / track * gridHeight;
    const seg = segments[segmentAt(gridY)];
    if (clamped - seg.y < SNAP) gridY = seg.top;
    scrollToGrid(gridY);
  };
  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || !active) return;
    // No text selection, and no focus ring: the rail takes focus from the keyboard only.
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragging(true);
    jump(trackY(event));
  };
  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const y = trackY(event);
    setHover(y);
    if (!dragging) return;
    pending.current = y;
    if (!dragFrame.current) dragFrame.current = requestAnimationFrame(() => {
      dragFrame.current = 0;
      if (pending.current !== null) jump(pending.current);
    });
  };
  const endDrag = (event: PointerEvent<HTMLDivElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
    setDragging(false);
  };
  useEffect(() => () => cancelAnimationFrame(dragFrame.current), []);
  // Up and down move to the start of the previous or next month.
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if ((event.key !== "ArrowUp" && event.key !== "ArrowDown") || !segments.length || !geometry) return;
    event.preventDefault();
    const top = Math.max(0, viewTop()), i = segmentAt(top);
    const target = event.key === "ArrowDown" ? segments[i + 1]?.top
      : top - segments[i].top > geometry.rowHeight / 2 ? segments[i].top : segments[i - 1]?.top;
    if (target !== undefined) scrollToGrid(target);
  };
  const hoverSegment = segments.length ? segments[segmentAt(Math.max(0, (Math.min(PAD_TOP + track, Math.max(PAD_TOP, hoverAt)) - PAD_TOP) / (track || 1) * gridHeight))] : null;
  const shown = active && segments.length > 0;
  const tip = "pointer-events-none absolute right-[calc(100%+8px)] flex h-7 max-w-[calc(100vw-72px)] items-center truncate whitespace-nowrap rounded-md bg-foreground px-2.5 text-xs font-medium text-background tabular-nums shadow-md transition-opacity duration-(--duration-fast)";
  return <motion.div ref={root} data-testid="timeline" data-active={shown || undefined} role="slider" tabIndex={shown ? 0 : -1}
    aria-label={t("timeline.label")} aria-valuemin={0} aria-valuemax={Math.max(0, segments.length - 1)} aria-valuenow={current ?? 0}
    aria-valuetext={current !== null && segments[current] ? label(segments[current].month) : undefined} aria-hidden={!shown || undefined}
    initial={false} animate={{ opacity: shown ? 1 : 0, x: shown ? 0 : 12 }} transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
    onPointerDown={onPointerDown} onMouseDown={(event) => event.preventDefault()} onPointerMove={onPointerMove} onPointerUp={endDrag} onPointerCancel={endDrag} onPointerLeave={() => { if (!dragging) setHover(null); }}
    onKeyDown={onKeyDown}
    className={cn("sticky w-12 touch-none select-none rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60",
      shown ? "pointer-events-auto" : "pointer-events-none", dragging ? "cursor-grabbing" : "cursor-pointer")}
    style={{ top: frame.inset, height: frame.height }}>
    {shown && <>
      {segments.map((seg) => <div key={`${seg.month.year}-${seg.month.month}`} data-testid="timeline-segment" data-start={seg.month.start} data-count={seg.month.count}
        className="absolute inset-x-0" style={{ top: seg.y, height: seg.h }}>
        {seg.year && <span data-testid="timeline-year" className="absolute right-2 bottom-full mb-0.5 text-[11px] leading-[14px] font-medium text-muted-foreground tabular-nums">{yearName(seg.month)}</span>}
        {seg.dot && <span data-testid="timeline-dot" className="absolute top-0 right-2 size-1 -translate-y-1/2 rounded-full bg-muted-foreground/50" />}
      </div>)}
      <div ref={indicator} data-testid="timeline-indicator" className="absolute top-0 right-0 h-8 w-0.5 rounded-full bg-brand" style={{ transform: `translateY(${indicatorY() - INDICATOR / 2}px)` }}>
        <span data-testid="timeline-scroll-label" aria-hidden className={cn(tip, "top-1/2 -translate-y-1/2", scrolling && hover === null ? "opacity-100" : "opacity-0")}>
          {current !== null && segments[current] ? label(segments[current].month) : ""}
        </span>
      </div>
      <div data-testid="timeline-hover" aria-hidden className={cn("pointer-events-none absolute inset-x-0 transition-opacity duration-(--duration-fast)", hover !== null ? "opacity-100" : "opacity-0")}
        style={{ top: hoverAt }}>
        <div className="h-px w-full bg-foreground/40" />
        <span data-testid="timeline-label" className={cn(tip, "top-0 -translate-y-1/2")} style={{ top: labelCenter(hoverAt, frame.height) - hoverAt }}>{hoverSegment ? label(hoverSegment.month) : ""}</span>
      </div>
    </>}
  </motion.div>;
}
