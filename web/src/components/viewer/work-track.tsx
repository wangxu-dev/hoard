import { useEffect, useRef, useState, type ReactNode } from "react";
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
  const size = Math.min(5, cursor.total);
  const loop = cursor.total > size;
  const [position, setPosition] = useState({ index, slot: loop ? 2 : index });
  const initial = useRef(position.slot);
  const positionRef = useRef(position);
  const latest = useRef(onSelect);
  latest.current = onSelect;
  const slots = Array.from({ length: size }, (_, slot) => {
    if (!loop) return slot;
    let distance = slot - position.slot;
    if (distance > 2) distance -= size;
    if (distance < -2) distance += size;
    return position.index + distance;
  });
  useMouseAxis(root);

  useEffect(() => {
    let live = true;
    for (const n of slots) void cursor.at(n).then(() => { if (live) redraw((v) => v + 1); }).catch(() => undefined);
    return () => { live = false; };
  }, [cursor, position.index]);

  useEffect(() => { onApi(api); return () => onApi(undefined); }, [api, onApi]);
  useEffect(() => {
    if (!api) return;
    let live = true;
    let request = 0;
    const select = () => {
      const previous = positionRef.current;
      const slot = api.selectedScrollSnap();
      let distance = slot - previous.slot;
      if (loop && distance > 2) distance -= size;
      if (loop && distance < -2) distance += size;
      const next = previous.index + distance;
      if (!distance) return;
      if (next < 0 || next >= cursor.total) {
        api.scrollTo(previous.slot);
        return;
      }
      const selected = { index: next, slot };
      positionRef.current = selected;
      setPosition(selected);
      const currentRequest = ++request;
      const apply = (value: LibraryEntry | undefined) => {
        if (!live || currentRequest !== request) return;
        if (value) latest.current(next, value);
        else {
          positionRef.current = previous;
          setPosition(previous);
          api.scrollTo(previous.slot);
        }
      };
      const cached = cursor.peek(next);
      if (cached) apply(cached);
      else void cursor.at(next).then(apply).catch(() => apply(undefined));
    };
    api.on("select", select);
    return () => { live = false; request++; api.off("select", select); };
  }, [api, cursor, loop, size]);

  return <div ref={root} className="absolute inset-0" data-testid="viewer-work-track">
    <Carousel orientation="vertical" setApi={setApi} plugins={plugins} onKeyDownCapture={undefined}
      opts={{ startIndex: initial.current, duration: reduced ? 0 : carouselDuration, loop, skipSnaps: false, watchSlides: false, watchDrag }} className="size-full">
      <CarouselContent viewportClassName="size-full touch-none" className="mt-0 h-full" data-testid="viewer-work-slides">
        {slots.map((n, slot) => <CarouselItem key={slot} className="relative min-h-0 h-full pt-0" aria-hidden={n !== index}>
          {n === index ? children : <img src={cursor.peek(Math.max(0, Math.min(n, cursor.total - 1)))?.cover ?? entry?.cover} alt="" draggable={false} className="size-full object-contain" />}
        </CarouselItem>)}
      </CarouselContent>
    </Carousel>
  </div>;
}
