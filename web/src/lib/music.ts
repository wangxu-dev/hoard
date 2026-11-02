import { useSyncExternalStore } from "react";
import { getLibrarySource, type PlayMode } from "./library";
import type { ListCursor } from "./library-cursor";
import { setLibraryFilters } from "./library-filters";

/** The app-wide music player: the list it was started from, its place, and how it moves on. */
export interface MusicState {
  queue: { cursor: ListCursor; index: number } | null;
  /** list loops the list, one loops the current sound, shuffle picks at random. */
  mode: PlayMode;
  /** The sound playing now, and whether it is paused, for indicators elsewhere in the app. */
  playing: { id: string; paused: boolean } | null;
  /** Counts requests to show the playing sound in the library. */
  locate: number;
}

let state: MusicState = { queue: null, mode: "list", playing: null, locate: 0 };
let history: number[] = [];
let loaded = false;
const listeners = new Set<() => void>();
function set(patch: Partial<MusicState>) { state = { ...state, ...patch }; listeners.forEach((fn) => fn()); }

function load() {
  if (loaded) return;
  loaded = true;
  const source = getLibrarySource();
  void source.playMode().then((mode) => set({ mode })).catch(() => undefined);
}

export function useMusic(): MusicState {
  load();
  return useSyncExternalStore((fn) => { listeners.add(fn); return () => { listeners.delete(fn); }; }, () => state);
}

export function playMusic(cursor: ListCursor, index: number) {
  load();
  history = [];
  set({ queue: { cursor, index } });
}

export function closeMusic() {
  history = [];
  set({ queue: null, playing: null });
}

export function setNowPlaying(playing: MusicState["playing"]) {
  if (playing?.id === state.playing?.id && playing?.paused === state.playing?.paused) return;
  set({ playing });
}

function go(index: number) {
  if (!state.queue) return;
  set({ queue: { ...state.queue, index } });
}

/** nextIndex is where the player goes after the current sound; -1 means stop. */
export function nextIndex(s: MusicState): number {
  if (!s.queue) return -1;
  const { cursor, index } = s.queue;
  const total = cursor.total;
  if (total <= 1) return total === 1 ? 0 : -1;
  if (s.mode === "shuffle") {
    // Any other sound of the list, never the same one twice in a row.
    const pick = Math.floor(Math.random() * (total - 1));
    return pick >= index ? pick + 1 : pick;
  }
  if (index + 1 < total) return index + 1;
  return 0;
}

export function playNext() {
  const next = nextIndex(state);
  if (next < 0 || !state.queue) return;
  history.push(state.queue.index);
  if (history.length > 200) history.shift();
  go(next);
}

export function playPrevious() {
  if (!state.queue) return;
  // Shuffle walks back through what was played; in order it steps one back.
  const back = history.pop();
  if (back !== undefined) { go(back); return; }
  const { index, cursor } = state.queue;
  if (index > 0) go(index - 1);
  else if (cursor.total > 1) go(cursor.total - 1);
}

export function canGoBack(s: MusicState): boolean {
  if (!s.queue) return false;
  return history.length > 0 || s.queue.cursor.total > 1;
}

/** locateNowPlaying opens the list the music was started from and asks the grid to show the playing sound. */
export function locateNowPlaying() {
  if (!state.queue) return;
  const { scope, query } = state.queue.cursor.query;
  setLibraryFilters({ scope, kind: "music", query, queryInput: query });
  set({ locate: state.locate + 1 });
}

const modes: PlayMode[] = ["list", "one", "shuffle"];

/** cyclePlayMode steps list loop, single loop, shuffle, and back. */
export function cyclePlayMode() {
  const mode = modes[(modes.indexOf(state.mode) + 1) % modes.length];
  set({ mode });
  void getLibrarySource().setPlayMode(mode).catch(() => undefined);
}
