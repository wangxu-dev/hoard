import type { CameraState } from "./types";

// Keep the real galactic disk overscanning the viewport, not shrinking to a framed coin.
// Perspective conservatism covers the far half; pitch and aspect both affect the floor.
export function minimumImmersiveZoom(width: number, height: number, pitch: number): number {
  const scale = Math.min(Math.max(1, width) * 0.375, Math.max(1, height) * 0.59);
  const tilt = Math.max(0.22, Math.abs(Math.cos(pitch)));
  return Math.min(9, Math.max(1.3, 1.06 * Math.hypot((width * 0.5) / scale, (height * 0.515) / (scale * tilt)) / (1.4 * 0.82)));
}

export function boundCamera(camera: CameraState, width: number, height: number): CameraState {
  return { ...camera, zoom: Math.max(minimumImmersiveZoom(width, height, camera.pitch), Math.min(9, camera.zoom)) };
}
