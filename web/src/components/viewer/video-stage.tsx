import { useEffect, useRef, useState } from "react";
import { useIsPresent } from "motion/react";
import type { LibraryEntry, MediaResult } from "@/lib/library";
import { claimPlayback, releaseMedia, togglePlay, useApplySound } from "@/lib/player";

/** VideoStage plays one video, trying each candidate URL and then asking for fresh ones. */
export function VideoStage({ entry, media, onElement, onExhausted, onDoubleClick }: {
  entry: LibraryEntry; media: MediaResult | null;
  onElement: (element: HTMLVideoElement | null) => void; onExhausted: (reason: string) => void; onDoubleClick: () => void;
}) {
  const [element, setElement] = useState<HTMLVideoElement | null>(null);
  const [candidate, setCandidate] = useState(0);
  const resume = useRef(0);
  // Some posts are a sound over a still picture; their stream has no video track.
  const [audioOnly, setAudioOnly] = useState(false);
  const fail = (element: HTMLVideoElement, reason: string) => {
    if (!src) return;
    resume.current = element.currentTime || resume.current;
    if (media && candidate + 1 < media.videos.length) setCandidate(candidate + 1); else onExhausted(reason);
  };
  const present = useIsPresent();
  useApplySound(element);
  useEffect(() => { if (present) onElement(element); }, [element, present, onElement]);
  useEffect(() => () => releaseMedia(element), [element]);
  useEffect(() => { if (!present) element?.pause(); }, [present, element]);
  useEffect(() => { setCandidate(0); }, [media]);
  const src = media?.videos[candidate];
  useEffect(() => {
    if (!element || !src || !present) return;
    claimPlayback(element);
    void element.play().catch(() => undefined);
  }, [element, src]);
  return <div className="relative size-full">
    <video ref={setElement} data-testid="viewer-video" src={src} poster={entry.cover} playsInline loop preload="auto"
    className="size-full object-contain"
    onClick={() => togglePlay(element)} onDoubleClick={onDoubleClick}
    onLoadedMetadata={(event) => { if (resume.current) { event.currentTarget.currentTime = resume.current; resume.current = 0; } }}
    // A stream whose picture WebKit cannot decode still plays its sound; treat it as failed.
    onLoadedData={(event) => setAudioOnly(event.currentTarget.videoWidth === 0)}
    onError={(event) => fail(event.currentTarget, "UNAVAILABLE")} />
    {audioOnly && <img data-testid="audio-cover" src={entry.cover} alt="" draggable={false} className="pointer-events-none absolute inset-0 size-full object-contain" />}
  </div>;
}
