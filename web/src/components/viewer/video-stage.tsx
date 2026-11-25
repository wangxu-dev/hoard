import { useEffect, useRef, useState } from "react";
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
  const videoKey = `${entry.id}:${media?.partId ?? ""}`;
  const sourceKey = `${videoKey}:${qualityKey ?? ""}`;
  const [candidateState, setCandidateState] = useState({ key: sourceKey, index: 0 });
  const candidate = candidateState.key === sourceKey ? candidateState.index : 0;
  const resume = useRef(0);
  const playbackRate = useRef<number | null>(null);
  const wasPlaying = useRef<boolean | null>(null);
  // Some posts are a sound over a still picture; their stream has no video track.
  const [audioOnly, setAudioOnly] = useState(false);
  const src = videos[candidate];
  const fail = (el: HTMLVideoElement, reason: string) => {
    if (!src) return;
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
  useEffect(() => {
    const sameVideo = prevVideoKey.current === videoKey;
    const isQualitySwitch = sameVideo && Boolean(prevQualityKey.current) && Boolean(qualityKey) && prevQualityKey.current !== qualityKey;
    const mediaRefreshed = sameVideo && !isQualitySwitch && prevMedia.current !== media;
    if (!sameVideo) {
      resume.current = 0;
      playbackRate.current = null;
      wasPlaying.current = null;
      setAudioOnly(false);
    } else if (isQualitySwitch && element) {
      resume.current = element.currentTime || resume.current;
      playbackRate.current = playbackRate.current ?? element.playbackRate;
      wasPlaying.current = wasPlaying.current ?? !element.paused;
      setAudioOnly(false);
    }
    prevVideoKey.current = videoKey;
    prevQualityKey.current = qualityKey;
    prevMedia.current = media;
    if (mediaRefreshed && candidate === 0) element?.load();
  }, [media, videoKey, qualityKey, element, candidate]);
  useEffect(() => {
    if (!element || !src || !present) return;
    const shouldPlay = wasPlaying.current ?? true;
    wasPlaying.current = null;
    if (!shouldPlay) return;
    claimPlayback(element);
    void element.play().catch(() => undefined);
  }, [element, src, media, present]);
  return <div className="relative size-full">
    <video ref={setElement} data-testid="viewer-video" src={src} poster={entry.cover} playsInline loop={!onNextPart && !media?.previewMs} onEnded={media?.previewMs ? undefined : onNextPart} preload="auto"
    className="size-full object-contain"
    onClick={() => togglePlay(element)} onDoubleClick={onDoubleClick}
    onLoadedMetadata={(event) => {
      if (resume.current) { event.currentTarget.currentTime = resume.current; resume.current = 0; }
      if (playbackRate.current !== null) { event.currentTarget.playbackRate = playbackRate.current; playbackRate.current = null; }
    }}
    // A stream whose picture WebKit cannot decode still plays its sound; treat it as failed.
    onLoadedData={(event) => setAudioOnly(event.currentTarget.videoWidth === 0)}
    onError={(event) => fail(event.currentTarget, "UNAVAILABLE")} />
    {audioOnly && <img data-testid="audio-cover" src={entry.cover} alt="" draggable={false} className="pointer-events-none absolute inset-0 size-full object-contain" />}
  </div>;
}
