import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useIsPresent } from "motion/react";
import type { LibraryEntry, MediaResult } from "@/lib/library";
import { claimPlayback, releaseMedia, togglePlay, useApplySound } from "@/lib/player";

/** VideoStage plays one video, trying each candidate URL and then asking for fresh ones. */
export function VideoStage({ entry, media, videos, qualityKey, onElement, onExhausted, onDoubleClick, onNextPart }: {
  entry: LibraryEntry; media: MediaResult | null; videos: string[]; qualityKey?: string;
  onNextPart?: () => void;
  onElement: (element: HTMLVideoElement | null) => void; onExhausted: (reason: string) => void; onDoubleClick: () => void;
}) {
  const [element, setElement] = useState<HTMLVideoElement | null>(null);
  const freezeCanvas = useRef<HTMLCanvasElement | null>(null);
  const [frozen, setFrozen] = useState(false);
  const videoKey = `${entry.id}:${media?.partId ?? ""}`;
  const sourceKey = `${videoKey}:${qualityKey ?? ""}`;
  const [candidateState, setCandidateState] = useState({ key: sourceKey, index: 0 });
  const candidate = candidateState.key === sourceKey ? candidateState.index : 0;
  const targetSrc = videos[candidate];
  const [activeSrc, setActiveSrc] = useState<string | undefined>(targetSrc);
  const resume = useRef(0);
  const waitingSeek = useRef(false);
  const playbackRate = useRef<number | null>(null);
  const wasPlaying = useRef<boolean | null>(null);
  // Some posts are a sound over a still picture; their stream has no video track.
  const [audioOnly, setAudioOnly] = useState(false);
  const fail = (el: HTMLVideoElement, reason: string) => {
    if (!activeSrc) return;
    setFrozen(false);
    waitingSeek.current = false;
    resume.current = el.currentTime || resume.current;
    playbackRate.current = playbackRate.current ?? el.playbackRate;
    if (media && candidate + 1 < videos.length) setCandidateState({ key: sourceKey, index: candidate + 1 });
    else onExhausted(reason);
  };
  const present = useIsPresent();
  useApplySound(element);
  useEffect(() => { if (present) onElement(element); }, [element, present, onElement]);
  useEffect(() => () => releaseMedia(element), [element]);
  useEffect(() => { if (!present) element?.pause(); }, [present, element]);
  const prevVideoKey = useRef(videoKey);
  const prevQualityKey = useRef(qualityKey);
  const prevMedia = useRef(media);
  useLayoutEffect(() => {
    const sameVideo = prevVideoKey.current === videoKey;
    const isQualitySwitch = sameVideo && Boolean(prevQualityKey.current) && Boolean(qualityKey) && prevQualityKey.current !== qualityKey;
    const mediaRefreshed = sameVideo && !isQualitySwitch && prevMedia.current !== media;
    if (!sameVideo) {
      resume.current = 0;
      waitingSeek.current = false;
      playbackRate.current = null;
      wasPlaying.current = null;
      setFrozen(false);
      setAudioOnly(false);
    } else if (isQualitySwitch && element && targetSrc && targetSrc !== activeSrc) {
      // Capture the exact current frame and playback position before swapping <video src>.
      resume.current = element.currentTime || resume.current;
      waitingSeek.current = resume.current > 0;
      playbackRate.current = playbackRate.current ?? element.playbackRate;
      wasPlaying.current = wasPlaying.current ?? !element.paused;
      setAudioOnly(false);
      const canvas = freezeCanvas.current;
      if (canvas && element.videoWidth > 0 && element.videoHeight > 0 && element.readyState >= 2) {
        canvas.width = element.videoWidth;
        canvas.height = element.videoHeight;
        const ctx = canvas.getContext("2d");
        if (ctx) {
          try {
            ctx.drawImage(element, 0, 0, canvas.width, canvas.height);
            setFrozen(true);
          } catch {
            setFrozen(false);
          }
        }
      }
    }
    prevVideoKey.current = videoKey;
    prevQualityKey.current = qualityKey;
    prevMedia.current = media;
    if (targetSrc !== activeSrc) {
      setActiveSrc(targetSrc);
    } else if (mediaRefreshed && candidate === 0) {
      element?.load();
    }
  }, [media, videoKey, qualityKey, element, candidate, targetSrc, activeSrc]);
  useEffect(() => {
    if (!frozen) return;
    // Safety valve so the freeze overlay can never get stuck if a stream stalls.
    const timer = window.setTimeout(() => { setFrozen(false); waitingSeek.current = false; }, 2500);
    return () => window.clearTimeout(timer);
  }, [frozen, activeSrc]);
  useEffect(() => {
    if (!element || !activeSrc || !present) return;
    const shouldPlay = wasPlaying.current ?? true;
    wasPlaying.current = null;
    if (!shouldPlay) return;
    claimPlayback(element);
    void element.play().catch(() => undefined);
  }, [element, activeSrc, media, present]);
  return <div className="relative size-full">
    <video ref={setElement} data-testid="viewer-video" src={activeSrc} poster={frozen ? undefined : entry.cover} playsInline loop={!onNextPart && !media?.previewMs} onEnded={media?.previewMs ? undefined : onNextPart} preload="auto"
    className="size-full object-contain"
    onClick={() => togglePlay(element)} onDoubleClick={onDoubleClick}
    onLoadedMetadata={(event) => {
      if (resume.current) { event.currentTarget.currentTime = resume.current; resume.current = 0; }
      if (playbackRate.current !== null) { event.currentTarget.playbackRate = playbackRate.current; playbackRate.current = null; }
    }}
    onSeeked={() => { waitingSeek.current = false; setFrozen(false); }}
    // A stream whose picture WebKit cannot decode still plays its sound; treat it as failed.
    onLoadedData={(event) => {
      setAudioOnly(event.currentTarget.videoWidth === 0);
      if (!waitingSeek.current) setFrozen(false);
    }}
    onError={(event) => fail(event.currentTarget, "UNAVAILABLE")} />
    <canvas ref={freezeCanvas} aria-hidden="true" data-testid="quality-freeze-frame"
      className={`pointer-events-none absolute inset-0 size-full object-contain transition-opacity duration-150 ${frozen ? "opacity-100" : "opacity-0"}`} />
    {audioOnly && <img data-testid="audio-cover" src={entry.cover} alt="" draggable={false} className="pointer-events-none absolute inset-0 size-full object-contain" />}
  </div>;
}
