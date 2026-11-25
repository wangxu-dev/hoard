import type { CameraState } from "./types";

export interface EntranceLayers {
  nebula: number;
  stars: number;
  covers: number;
  active: boolean;
}

export interface RememberedView {
  camera: CameraState;
  rotation: number;
  paused: boolean;
}

// Pure timeline/state helpers: no content IDs, graph changes, or offscreen spin.
const smooth = (t: number): number => {
  t = Math.max(0, Math.min(1, t));
  return t * t * (3 - 2 * t);
};

export function entranceLayers(elapsed: number, reduced = false): EntranceLayers {
  if (reduced) return { nebula: 1, stars: 1, covers: 1, active: false };
  return {
    nebula: 0.36 + 0.64 * smooth(elapsed / 420),
    stars: smooth((elapsed - 40) / 480),
    covers: smooth((elapsed - 180) / 480),
    active: elapsed < 660,
  };
}

let remembered: RememberedView | null = null;

export function rememberView(camera: CameraState, rotation: number, paused: boolean): void {
  remembered = { camera: { ...camera }, rotation, paused };
}

export function recallView(): RememberedView | null {
  return remembered ? { ...remembered, camera: { ...remembered.camera } } : null;
}

export function landingCamera(camera: CameraState): CameraState {
  return { ...camera, zoom: camera.zoom * 1.035, pitch: Math.min(1.35, camera.pitch + 0.022) };
}
