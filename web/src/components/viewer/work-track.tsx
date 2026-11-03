import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { useReducedMotion } from "motion/react";
import { WheelGesturesPlugin } from "embla-carousel-wheel-gestures";
import { Carousel, CarouselContent, CarouselItem, type CarouselApi } from "@/components/ui/carousel";
import { carouselDuration } from "@/lib/motion";
import type { LibraryEntry } from "@/lib/library";
import type { ListCursor } from "@/lib/library-cursor";

const controls = 'button, a, input, [role="slider"], [data-preview-actions], [data-testid="viewer-controls"]';
const plugins = [WheelGesturesPlugin()];
const watchDrag = (_: unknown, event: MouseEvent | TouchEvent) => {
  const target = event.target as Element;
  return !target.closest(controls) && !(event.type === "mousedown" && target.closest('[data-testid="viewer-gallery"]'));
};

/** Embla locks touch axes itself; mouse drags need the same arbitration for nested carousels. */
function useMouseAxis(root: React.RefObject<HTMLDivElement | null>) {
  useEffect(() => {
    const node = root.current;
    if (!node) return;
    let origin: MouseEvent | undefined;
    let target: Element | null = null;
    let dragging = false;
    const down = (event: MouseEvent) => {
      if (!event.isTrusted || event.button !== 0 || (event.target as Element).closest(controls)) return;
      event.stopPropagation();
      origin = event;
      target = event.target as Element;
    };
    const move = (event: MouseEvent) => {
      if (!origin) return;
      const x = Math.abs(event.clientX - origin.clientX), y = Math.abs(event.clientY - origin.clientY);
      if (Math.max(x, y) < 8) return;
      const viewport = x > y ? target?.closest('[data-testid="viewer-gallery"] [data-slot="carousel-content"]') : node.querySelector('[data-slot="carousel-content"]');
      if (viewport) {
        viewport.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, cancelable: true, clientX: origin.clientX, clientY: origin.clientY, button: 0, buttons: 1 }));
        dragging = true;
      }
      origin = undefined;
    };
    const end = () => { origin = undefined; };
    const cancel = () => { origin = undefined; if (dragging) document.dispatchEvent(new MouseEvent("mouseup", { bubbles: true })); dragging = false; };
    const click = (event: MouseEvent) => { if (dragging) { dragging = false; event.preventDefault(); event.stopPropagation(); } };
    node.addEventListener("mousedown", down, true);
    document.addEventListener("mousemove", move, true);
    document.addEventListener("mouseup", end);
    window.addEventListener("blur", cancel);
    node.addEventListener("click", click, true);
    return () => {
      cancel();
      node.removeEventListener("mousedown", down, true);
      document.removeEventListener("mousemove", move, true);
      document.removeEventListener("mouseup", end);
      window.removeEventListener("blur", cancel);
      node.removeEventListener("click", click, true);
    };
  }, [root]);
}

export function WorkTrack({ cursor, index, entry, onSelect, onApi, children }: {
  cursor: ListCursor; index: number; entry: LibraryEntry | undefined;
  onSelect: (index: number, entry: LibraryEntry) => void;
  onApi: (api: CarouselApi) => void;
  children: ReactNode;
}) {
  const root = useRef<HTMLDivElement>(null);
  const [api, setApi] = useState<CarouselApi>();
  const [, redraw] = useState(0);
  const reduced = useReducedMotion();
  const initial = useRef(index > 0 ? 1 : 0);
  const slots = [index - 1, index, index + 1].filter((n) => n >= 0 && n < cursor.total);
  const active = slots.indexOf(index);
  const latest = useRef({ slots, index, onSelect });
  latest.current = { slots, index, onSelect };
  const committing = useRef(false);
  useMouseAxis(root);

  useEffect(() => {
    let live = true;
    for (const n of slots) void cursor.at(n).then(() => { if (live) redraw((v) => v + 1); }).catch(() => undefined);
    return () => { live = false; };
  }, [cursor, index]);

  useLayoutEffect(() => {
    if (!api) return;
    // Keep slot nodes stable so regular rebasing retains the wheel plugin's momentum state.
    if (api.slideNodes().length !== slots.length) api.reInit();
    api.scrollTo(active, true);
  }, [api, index, slots.length, active]);

  useEffect(() => { onApi(api); return () => onApi(undefined); }, [api, onApi]);
  useEffect(() => {
    if (!api) return;
    let live = true;
    const settle = () => {
      const { slots, index, onSelect } = latest.current;
      const next = slots[api.selectedScrollSnap()];
      if (next === undefined || next === index || committing.current) return;
      committing.current = true;
      void cursor.at(next).then((value) => {
        if (!live) return;
        if (value) onSelect(next, value);
        else api.scrollTo(slots.indexOf(index));
      }).catch(() => { if (live) api.scrollTo(slots.indexOf(index)); }).finally(() => { committing.current = false; });
    };
    api.on("settle", settle);
    return () => { live = false; api.off("settle", settle); };
  }, [api, cursor]);

  return <div ref={root} className="absolute inset-0" data-testid="viewer-work-track">
    <Carousel orientation="vertical" setApi={setApi} plugins={plugins} onKeyDownCapture={undefined}
      opts={{ startIndex: initial.current, duration: reduced ? 0 : carouselDuration, skipSnaps: false, watchSlides: false, watchDrag }} className="size-full">
      <CarouselContent viewportClassName="size-full touch-none" className="mt-0 h-full" data-testid="viewer-work-slides">
        {slots.map((n, slot) => <CarouselItem key={slot} className="relative min-h-0 h-full pt-0" aria-hidden={n !== index}>
          {n === index ? children : <img src={cursor.peek(n)?.cover ?? entry?.cover} alt="" draggable={false} className="size-full object-contain" />}
        </CarouselItem>)}
      </CarouselContent>
    </Carousel>
  </div>;
}
