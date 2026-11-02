import { forwardRef, useRef, useState, type ComponentProps, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Volume, Volume1, Volume2, VolumeX } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { DropdownMenu, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { formatClock, setSound, useSound, type MediaState } from "@/lib/player";

export const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2];

/** ControlButton is a round icon button for dark media surfaces, with a tooltip. */
export const ControlButton = forwardRef<HTMLButtonElement, ComponentProps<typeof Button> & { label: string; hint?: string; side?: "top" | "bottom" | "left" | "right" }>(
  function ControlButton({ label, hint, side = "top", className, children, ...props }, ref) {
    return <Tooltip>
      <TooltipTrigger asChild>
        <Button ref={ref} variant="ghost" size="icon" aria-label={label} className={cn("size-9 rounded-full text-viewer-foreground transition-[color,background-color,scale] duration-(--duration-fast) ease-(--ease-out) active:scale-90 motion-reduce:active:scale-100 hover:bg-viewer-control-hover hover:text-viewer-foreground focus-visible:ring-viewer-foreground/30 [&_svg:not([class*='size-'])]:size-[18px]", className)} {...props}>{children}</Button>
      </TooltipTrigger>
      <TooltipContent side={side} sideOffset={6}>{label}{hint && <kbd className="ml-2 font-sans opacity-60">{hint}</kbd>}</TooltipContent>
    </Tooltip>;
  });

/** SeekBar is a slim progress line with the buffered range and the time under the pointer. */
export function SeekBar({ element, state, className }: { element: HTMLMediaElement | null; state: MediaState; className?: string }) {
  const { t } = useTranslation("library");
  const [drag, setDrag] = useState<number | null>(null);
  const [hover, setHover] = useState<{ x: number; time: number } | null>(null);
  const box = useRef<HTMLDivElement>(null);
  const max = state.duration;
  const value = Math.min(drag ?? state.time, max || 1);
  const seek = (next: number) => { if (element && Number.isFinite(next)) element.currentTime = next; };
  return <div ref={box} data-testid="seek-bar" className={cn("group/seek relative", className)}
    onPointerMove={(event) => {
      const rect = box.current?.getBoundingClientRect();
      if (!rect || !max) return;
      const x = Math.min(Math.max(event.clientX - rect.left, 0), rect.width);
      setHover({ x, time: (x / rect.width) * max });
    }}
    onPointerLeave={() => setHover(null)}>
    {hover && <div className="pointer-events-none absolute bottom-full mb-1.5 -translate-x-1/2 rounded-md bg-viewer-panel px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-viewer-foreground shadow-md backdrop-blur-md" style={{ left: hover.x }}>{formatClock(hover.time)}</div>}
    <Slider aria-label={t("viewer.progress")} min={0} max={max || 1} step={0.01} value={[value]} disabled={!max}
      onValueChange={([next]) => { setDrag(next); seek(next); }}
      onValueCommit={([next]) => { seek(next); setDrag(null); }}
      className="h-4 cursor-pointer data-[disabled]:opacity-100"
      trackClassName="bg-viewer-track data-[orientation=horizontal]:h-[3px] transition-[height] duration-150 group-hover/seek:data-[orientation=horizontal]:h-[5px]"
      rangeClassName="z-10 bg-viewer-foreground"
      thumbClassName={cn("z-20 size-3 border-0 bg-viewer-foreground shadow-md ring-viewer-foreground/25 transition-transform duration-150 hover:ring-4", drag === null && "scale-0 group-hover/seek:scale-100 focus-visible:scale-100")}>
      <div className="absolute inset-y-0 left-0 bg-viewer-buffer" style={{ width: `${max ? Math.min(100, (state.buffered / max) * 100) : 0}%` }} />
    </Slider>
  </div>;
}

export function TimeLabel({ state }: { state: MediaState }) {
  return <span className="text-xs font-medium tabular-nums text-viewer-foreground/90">{formatClock(state.time)}<span className="text-viewer-muted"> / {formatClock(state.duration)}</span></span>;
}

/** VolumeControl is a mute button that opens a slim volume slider on hover. Both persist. */
export function VolumeControl() {
  const { t } = useTranslation("library");
  const { volume, muted } = useSound();
  const level = muted ? 0 : volume;
  const Icon = level === 0 ? VolumeX : level < 0.34 ? Volume : level < 0.67 ? Volume1 : Volume2;
  return <div data-testid="volume" className="group/volume flex items-center">
    <ControlButton label={muted ? t("viewer.unmute") : t("viewer.mute")} hint="M" onClick={() => setSound({ muted: !muted, ...(muted && volume === 0 ? { volume: 0.5 } : {}) })}><Icon /></ControlButton>
    <div className="w-0 overflow-hidden opacity-0 transition-[width,opacity] duration-200 ease-out group-focus-within/volume:w-24 group-focus-within/volume:opacity-100 group-hover/volume:w-24 group-hover/volume:opacity-100">
      <Slider aria-label={t("viewer.volume")} min={0} max={1} step={0.01} value={[level]} onValueChange={([next]) => setSound({ volume: next, muted: next === 0 })}
        className="ml-1 h-6 w-[84px] cursor-pointer"
        trackClassName="bg-viewer-track data-[orientation=horizontal]:h-[3px]"
        rangeClassName="bg-viewer-foreground"
        thumbClassName="size-3 border-0 bg-viewer-foreground shadow-md ring-viewer-foreground/25" />
    </div>
  </div>;
}

export function SpeedMenu({ rate, onRate }: { rate: number; onRate: (rate: number) => void }) {
  const { t } = useTranslation("library");
  const label = (value: number) => `${value === 1 ? "1.0" : value}×`;
  return <DropdownMenu modal={false}>
    <Tooltip>
      <TooltipTrigger asChild>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" aria-label={t("viewer.speed")} className="h-9 min-w-12 rounded-full px-2.5 text-xs font-semibold tabular-nums text-viewer-foreground hover:bg-viewer-control-hover hover:text-viewer-foreground">{label(rate)}</Button>
        </DropdownMenuTrigger>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={6}>{t("viewer.speed")}</TooltipContent>
    </Tooltip>
    <DropdownMenuContent side="top" align="center" sideOffset={8} className="dark min-w-24">
      <DropdownMenuRadioGroup value={String(rate)} onValueChange={(value) => onRate(Number(value))}>
        {[...SPEEDS].reverse().map((value) => <DropdownMenuRadioItem key={value} value={String(value)} className="tabular-nums">{label(value)}</DropdownMenuRadioItem>)}
      </DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>;
}

export function ControlRow({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("flex items-center gap-1", className)}>{children}</div>;
}
