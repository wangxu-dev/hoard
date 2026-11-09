import { useEffect, useMemo, useRef, useState } from "react";
import { Pause, Play, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import type { ViewControls } from "./renderer";
import { clampRange, matchesPublication, monthDate, nextPlaybackRange, publicationBins, type DatedWork, type TimeSelection } from "./publication-time";

let remembered: TimeSelection = { range: null, includeUnknown: false };

export function PublicationTimeline({ works, controls }: { works: DatedWork[]; controls: ViewControls }) {
  const { t, i18n } = useTranslation("explore");
  const { bins, unknown } = useMemo(() => publicationBins(works), [works]);
  const first = bins[0]?.month ?? 0, last = bins.at(-1)?.month ?? first;
  const [selection, setSelection] = useState<TimeSelection>(() => ({ ...remembered, range: remembered.range && clampRange(remembered.range, first, last) }));
  const [playing, setPlaying] = useState(false);
  const [hoverMonth, setHoverMonth] = useState<number | null>(null);
  const section = useRef<HTMLElement>(null);
  const drag = useRef<{ x: number; range: [number, number]; width: number; moved: boolean; pointer: number } | null>(null);
  const range = selection.range ?? [first, last];
  const format = useMemo(() => new Intl.DateTimeFormat(i18n.language, { year: "numeric", month: "short" }), [i18n.language]);
  const label = (month: number) => format.format(monthDate(month));
  const count = useMemo(() => works.filter(work => matchesPublication(work, selection)).length, [works, selection]);
  const peak = Math.max(1, ...bins.map(bin => bin.count));
  const full = selection.range === null;
  useEffect(() => {
    section.current?.querySelectorAll('[role="slider"]').forEach((thumb, index) => {
      thumb.setAttribute("aria-label", t(index === 0 ? "timeline.start" : "timeline.end"));
      thumb.setAttribute("aria-valuetext", label(range[index]));
    });
  }, [range[0], range[1], i18n.language, t]);
  useEffect(() => {
    remembered = selection;
    controls.setPublicationRange(selection);
  }, [controls, selection]);
  useEffect(() => {
    if (!playing) return;
    const timer = window.setInterval(() => {
      setSelection(current => ({ ...current, range: nextPlaybackRange(current.range, first, last) }));
    }, Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--explore-playback-step")));
    const stop = () => { if (document.hidden) setPlaying(false); };
    document.addEventListener("visibilitychange", stop);
    window.addEventListener("blur", stopPlayback);
    function stopPlayback() { setPlaying(false); }
    return () => { clearInterval(timer); document.removeEventListener("visibilitychange", stop); window.removeEventListener("blur", stopPlayback); };
  }, [playing, first, last]);
  useEffect(() => { if (playing && range[1] >= last) setPlaying(false); }, [playing, range[1], last]);
  const choose = (next: [number, number]) => { setPlaying(false); setSelection(current => ({ ...current, range: clampRange(next, first, last) })); };
  const reset = () => { setPlaying(false); setSelection({ range: null, includeUnknown: false }); };
  const yearStep = bins.length > 96 ? 2 : 1;
  return <section ref={section} className="explore-timeline" aria-label={t("timeline.label")} data-playing={playing}>
    <div className="flex min-w-0 items-center gap-2">
      <div className="flex min-w-0 flex-1 items-baseline gap-3 text-xs">
        <span className="publication-caption shrink-0 text-muted-foreground">{t("timeline.published")}</span>
        <span className="truncate font-medium" data-testid="publication-range">{full ? t("timeline.all") : range[0] === range[1] ? label(range[0]) : `${label(range[0])} — ${label(range[1])}`}</span>
        <span className="publication-total shrink-0 text-muted-foreground" data-testid="publication-count">{t("timeline.count", { count })}</span>
      </div>
      {unknown > 0 && <Button variant="ghost" size="sm" aria-pressed={selection.includeUnknown} disabled={full} onClick={() => { setPlaying(false); setSelection(current => ({ ...current, includeUnknown: !current.includeUnknown })); }}>{t("timeline.unknown", { count: unknown })}</Button>}
      <Button variant="ghost" size="icon" className="size-8 shrink-0" disabled={bins.length < 2} aria-label={t(playing ? "timeline.pause" : "timeline.play")} title={t(playing ? "timeline.pause" : "timeline.play")} onClick={() => {
        if (playing) setPlaying(false);
        else { setSelection(current => ({ ...current, range: !current.range || current.range[1] >= last ? nextPlaybackRange(current.range, first, last) : current.range })); setPlaying(true); }
      }}>{playing ? <Pause /> : <Play />}</Button>
      <Button variant="ghost" size="icon" className="size-8 shrink-0" aria-label={t("timeline.reset")} title={t("timeline.reset")} disabled={full} onClick={reset}><RotateCcw /></Button>
    </div>
    {bins.length > 0 ? <>
      <div className="publication-density" role="group" aria-label={t("timeline.density")}
        onPointerDown={event => {
          if (event.button !== 0) return;
          setPlaying(false);
          drag.current = { x: event.clientX, range: [...range], width: event.currentTarget.clientWidth, moved: false, pointer: event.pointerId };
          event.currentTarget.setPointerCapture(event.pointerId);
        }}
        onPointerMove={event => {
          const rect = event.currentTarget.getBoundingClientRect();
          const index = Math.max(0, Math.min(bins.length - 1, Math.floor((event.clientX - rect.left) / rect.width * bins.length)));
          setHoverMonth(index);
          const start = drag.current;
          if (!start) return;
          if (Math.abs(event.clientX - start.x) > 4) start.moved = true;
          if (start.moved) {
            const delta = Math.round((event.clientX - start.x) / start.width * bins.length);
            choose([start.range[0] + delta, start.range[1] + delta]);
          }
        }}
        onPointerUp={event => {
          const start = drag.current;
          drag.current = null;
          if (!start) return;
          if (start.moved) { event.preventDefault(); }
          else {
            const rect = event.currentTarget.getBoundingClientRect();
            const index = Math.max(0, Math.min(bins.length - 1, Math.floor((event.clientX - rect.left) / rect.width * bins.length)));
            choose([bins[index].month, bins[index].month]);
          }
          if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
        }} onPointerCancel={() => { drag.current = null; }} onLostPointerCapture={() => { drag.current = null; }} onPointerLeave={() => setHoverMonth(null)}>
        {bins.map((bin, index) => <button key={bin.month} type="button" tabIndex={-1} className="publication-bin" data-selected={bin.month >= range[0] && bin.month <= range[1]} aria-label={t("timeline.monthCount", { month: label(bin.month), count: bin.count })} title={t("timeline.monthCount", { month: label(bin.month), count: bin.count })} onClick={event => { if (event.detail === 0) choose([bin.month, bin.month]); }}>
          <span style={{ height: `${bin.count ? Math.max(8, Math.sqrt(bin.count / peak) * 100) : 3}%` }} />
          {index === hoverMonth && <span className="publication-month-tip">{label(bin.month)} · {bin.count}</span>}
        </button>)}
      </div>
      <Slider min={first} max={last === first ? first + 1 : last} step={1} minStepsBetweenThumbs={0} value={range} disabled={bins.length < 2} onValueChange={value => choose([value[0], value[1]])} className="publication-slider" aria-label={t("timeline.range")} onFocusCapture={event => {
        if (event.target.getAttribute("role") === "slider") setPlaying(false);
      }} />
      <div className="publication-years" aria-hidden="true">{bins.map((bin, index) => (index === 0 || index === bins.length - 1 && bin.month % 12 === 0 || bin.month % 12 === 0 && Math.floor(bin.month / 12) % yearStep === 0) && <span key={bin.month} style={{ left: `${index / Math.max(1, bins.length - 1) * 100}%` }}>{Math.floor(bin.month / 12)}</span>)}</div>
      <span className="sr-only" role="status" aria-live={playing ? "off" : "polite"}>{t("timeline.count", { count })}</span>
    </> : <p className="py-2 text-xs text-muted-foreground">{t("timeline.noDates")}</p>}
  </section>;
}
