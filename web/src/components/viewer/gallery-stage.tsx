import { createPortal } from "react-dom";
import { useCallback, useEffect, useRef, useState } from "react";
import { useIsPresent } from "motion/react";
import { useTranslation } from "react-i18next";
import { Carousel, CarouselContent, CarouselItem, type CarouselApi } from "@/components/ui/carousel";
import { cn } from "@/lib/utils";
import type { LibraryEntry, MediaResult } from "@/lib/library";
import { claimPlayback, releaseMedia, useApplySound } from "@/lib/player";
import { warmImage } from "@/lib/library-cursor";

const AUTOPLAY_MS = 3000;

export interface GalleryHandle { move: (delta: number) => void }

/** GalleryStage shows the images of one gallery with its sound, advancing every 3 s until the viewer swipes. */
export function GalleryStage({ entry, media, onElement, onHandle, onSlide, onExpired, badgeSlot }: {
  entry: LibraryEntry; media: MediaResult | null;
  /** Where the live photo switch goes: the viewer top bar, beside the item counter. */
  badgeSlot: HTMLElement | null;
  onElement: (element: HTMLAudioElement | null) => void; onHandle: (handle: GalleryHandle | null) => void;
  onSlide: (index: number, count: number) => void;
  /** A sound or live photo failed to load; fresh URLs are asked for once. */
  onExpired: () => void;
}) {
  const { t } = useTranslation("library");
  const [api, setApi] = useState<CarouselApi>();
  const [selected, setSelected] = useState(0);
  const [manual, setManual] = useState(false);
  // Live photos play muted and looped on the current slide, like 动图; the badge turns them off.
  const [liveOff, setLiveOff] = useState(false);
  const [audio, setAudio] = useState<HTMLAudioElement | null>(null);
  const resume = useRef(0);
  const reached = useRef(new Set<number>([0, 1, 2]));
  const present = useIsPresent();
  const images = media?.images.length ? media.images : [{ url: entry.cover }];
  useApplySound(audio);
  useEffect(() => { if (present) onElement(audio); }, [audio, present, onElement]);
  useEffect(() => () => releaseMedia(audio), [audio]);
  useEffect(() => { if (!present) audio?.pause(); }, [present, audio]);
  useEffect(() => {
    if (!audio || !media?.music.url || !present) return;
    claimPlayback(audio);
    void audio.play().catch(() => undefined);
  }, [audio, media?.music.url]);
  // Keep the previous image and the next two loaded and decoded so a swipe never waits.
  for (let n = selected - 1; n <= selected + 2; n++) if (n >= 0 && n < images.length) reached.current.add(n);
  useEffect(() => { for (let n = selected - 1; n <= selected + 2; n++) warmImage(images[n]?.url); }, [selected, images]);
  useEffect(() => {
    if (!api) return;
    const sync = () => setSelected(api.selectedScrollSnap());
    const grab = () => setManual(true);
    sync();
    api.on("select", sync).on("reInit", sync).on("pointerDown", grab);
    return () => { api.off("select", sync).off("reInit", sync).off("pointerDown", grab); };
  }, [api]);
  useEffect(() => { onSlide(selected, images.length); }, [selected, images.length, onSlide]);
  const move = useCallback((delta: number) => {
    setManual(true);
    if (delta > 0) api?.scrollNext(); else api?.scrollPrev();
  }, [api]);
  useEffect(() => { if (present) onHandle({ move }); return () => onHandle(null); }, [move, present, onHandle]);
  useEffect(() => {
    if (manual || !api || !present || images.length < 2 || selected >= images.length - 1) return;
    const timer = setTimeout(() => api.scrollNext(), AUTOPLAY_MS);
    return () => clearTimeout(timer);
  }, [manual, api, present, images.length, selected]);
  const current = images[selected];
  const hasLive = !!current && "liveUrl" in current && !!current.liveUrl;
  return <div data-testid="viewer-gallery" className="relative size-full">
    {present && hasLive && badgeSlot && createPortal(
      <button type="button" aria-pressed={!liveOff} onClick={() => setLiveOff(!liveOff)}
        className="inline-flex items-center gap-1.5 rounded-full bg-viewer-control px-2.5 py-1 text-xs font-medium text-viewer-foreground backdrop-blur-md">
        <span className={cn("size-2 rounded-full border border-viewer-foreground", !liveOff && "bg-viewer-foreground")} />{t("viewer.livePhoto")}
      </button>, badgeSlot)}
    <Carousel setApi={setApi} opts={{ duration: 24, skipSnaps: false }} className="size-full">
      <CarouselContent viewportClassName="size-full" className="ml-0 h-full">
        {images.map((image, n) => <CarouselItem key={`${image.url}:${n}`} className="relative h-full pl-0" aria-label={`${n + 1} / ${images.length}`}>
          {reached.current.has(n) && <img src={image.url} alt="" draggable={false} decoding="async" loading="eager" className="size-full object-contain" />}
          {n === selected && !liveOff && present && "liveUrl" in image && image.liveUrl && <video data-testid="live-photo" src={image.liveUrl} autoPlay muted loop playsInline poster={image.url} onError={() => onExpired()} className="absolute inset-0 size-full object-contain" />}
        </CarouselItem>)}
      </CarouselContent>
    </Carousel>
    {media?.music.url && <audio ref={setAudio} src={media.music.url} loop preload="auto" className="hidden"
      onLoadedMetadata={(event) => { if (resume.current) { event.currentTarget.currentTime = resume.current; resume.current = 0; } }}
      onError={(event) => { resume.current = event.currentTarget.currentTime || resume.current; onExpired(); }} />}
  </div>;
}
