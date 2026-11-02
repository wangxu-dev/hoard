import { useEffect, useState, useSyncExternalStore } from "react";
import { getLibrarySource } from "./library";

/** Saved volume and mute shared by every player in the app. */
interface Sound { volume: number; muted: boolean }
let sound: Sound = { volume: 1, muted: false };
let loaded = false;
const listeners = new Set<() => void>();
let saveTimer: ReturnType<typeof setTimeout> | undefined;

function emit() { listeners.forEach((fn) => fn()); }

export function loadSound() {
  if (loaded) return;
  loaded = true;
  const source = getLibrarySource();
  void Promise.all([source.volume(), source.muted()]).then(([volume, muted]) => { sound = { volume, muted }; emit(); }).catch(() => undefined);
}

const dirty = { volume: false, muted: false };
export function setSound(next: Partial<Sound>) {
  sound = { ...sound, ...next };
  if (next.volume !== undefined) dirty.volume = true;
  if (next.muted !== undefined) dirty.muted = true;
  emit();
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    const source = getLibrarySource();
    if (dirty.volume) void source.setVolume(Math.min(1, Math.max(0, sound.volume))).catch(() => undefined);
    if (dirty.muted) void source.setMuted(sound.muted).catch(() => undefined);
    dirty.volume = dirty.muted = false;
  }, 250);
}

export function useSound(): Sound {
  useEffect(loadSound, []);
  return useSyncExternalStore((fn) => { listeners.add(fn); return () => listeners.delete(fn); }, () => sound);
}

/** Applies the shared sound to a media element. */
export function useApplySound(element: HTMLMediaElement | null) {
  const { volume, muted } = useSound();
  useEffect(() => { if (element) { element.volume = volume; element.muted = muted; } }, [element, volume, muted]);
}

/** Only one media element plays at a time across the app. */
let active: HTMLMediaElement | null = null;
export function claimPlayback(element: HTMLMediaElement) {
  if (active && active !== element && !active.paused) active.pause();
  active = element;
}
export function pauseAll() { if (active && !active.paused) active.pause(); }

/** releaseMedia stops a media element and frees its network and decoder. */
export function releaseMedia(element: HTMLMediaElement | null) {
  if (!element) return;
  element.pause();
  element.removeAttribute("src");
  element.load();
  if (active === element) active = null;
}

export interface MediaState { time: number; duration: number; buffered: number; paused: boolean; waiting: boolean; rate: number; ended: boolean }
const idle: MediaState = { time: 0, duration: 0, buffered: 0, paused: true, waiting: false, rate: 1, ended: false };

function bufferedEnd(el: HTMLMediaElement): number {
  const ranges = el.buffered;
  for (let i = 0; i < ranges.length; i++) if (ranges.start(i) <= el.currentTime + 0.5 && ranges.end(i) >= el.currentTime) return ranges.end(i);
  return 0;
}

/** useMediaState follows a media element, with a smooth clock while it plays. */
export function useMediaState(element: HTMLMediaElement | null): MediaState {
  const [state, setState] = useState<MediaState>(idle);
  useEffect(() => {
    if (!element) { setState(idle); return; }
    let frame = 0;
    const read = () => setState({
      time: element.currentTime, duration: Number.isFinite(element.duration) ? element.duration : 0,
      buffered: bufferedEnd(element), paused: element.paused, waiting: element.readyState < 3 && !element.paused,
      rate: element.playbackRate, ended: element.ended,
    });
    const tick = () => { read(); frame = element.paused ? 0 : requestAnimationFrame(tick); };
    const start = () => { if (!frame) frame = requestAnimationFrame(tick); read(); };
    const events = ["loadedmetadata", "durationchange", "progress", "pause", "waiting", "canplay", "ratechange", "seeked", "ended", "emptied", "timeupdate"];
    events.forEach((name) => element.addEventListener(name, read));
    element.addEventListener("play", start);
    element.addEventListener("playing", start);
    read();
    if (!element.paused) start();
    return () => {
      cancelAnimationFrame(frame);
      events.forEach((name) => element.removeEventListener(name, read));
      element.removeEventListener("play", start);
      element.removeEventListener("playing", start);
    };
  }, [element]);
  return state;
}

/** formatClock renders seconds as 1:05 or 1:02:05. */
export function formatClock(seconds: number): string {
  const total = Math.max(0, Math.floor(Number.isFinite(seconds) ? seconds : 0));
  const h = Math.floor(total / 3600), m = Math.floor((total % 3600) / 60), s = total % 60;
  return h ? `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}` : `${m}:${String(s).padStart(2, "0")}`;
}

export function togglePlay(element: HTMLMediaElement | null) {
  if (!element) return;
  if (element.paused || element.ended) { claimPlayback(element); void element.play().catch(() => undefined); } else element.pause();
}
