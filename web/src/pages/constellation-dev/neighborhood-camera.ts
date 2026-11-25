import H from "./core";
import { boundCamera } from "./camera-bounds";
import type { CameraState, PreparedWork } from "./types";

const clamp = (v: number, a: number, b: number): number => Math.max(a, Math.min(b, v));

export interface NeighborhoodFrame {
  camera: CameraState;
  neighborCount: number;
  reason: "already-visible" | "neighbor-bounds" | "isolated";
}

// A single snapshot of real graph geometry, never a per-frame tracking target.
export function frameNeighborhood(
  work: PreparedWork,
  byKey: Map<string, PreparedWork>,
  camera: CameraState,
  width: number,
  height: number,
  rotation = 0,
): NeighborhoodFrame {
  const neighbors = (work.neighbors || [])
    .map((n) => ({ work: byKey.get(n.key), weight: n.weight }))
    .filter((n): n is { work: PreparedWork; weight: number } => Boolean(n.work) && n.weight > 0)
    .slice(0, 36);
  const right = width < 760 ? width * 0.43 : Math.min(370, width * 0.3);
  const rect = { left: 28, top: 90, right: width - right - 24, bottom: height - 100 };
  const points = [{ work, weight: 2 }, ...neighbors];
  const project = (c: CameraState) => points.map((n) => ({ ...H.project(H.rotateZ(n.work.pos, rotation), c, width, height), weight: n.weight }));
  const inside = (p: { x: number; y: number }) => p.x >= rect.left && p.x <= rect.right && p.y >= rect.top && p.y <= rect.bottom;
  const before = project(camera);
  const mass = before.reduce((s, p) => s + p.weight, 0);
  const visible = before.reduce((s, p) => s + (inside(p) ? p.weight : 0), 0) / mass;
  const mostlyVisible = inside(before[0]) && visible >= 0.85;
  // No unrelated neighbors invented for isolates; only reveal them from behind UI.
  let zoom = camera.zoom;
  if (neighbors.length) {
    if (mostlyVisible) zoom *= 1.06;
    else {
      const xs = before.map((p) => p.x);
      const ys = before.map((p) => p.y);
      const fit = Math.min(
        ((rect.right - rect.left) * 0.8) / Math.max(90, Math.max(...xs) - Math.min(...xs)),
        ((rect.bottom - rect.top) * 0.8) / Math.max(90, Math.max(...ys) - Math.min(...ys)),
      );
      zoom *= clamp(fit, 0.92, 1.24);
    }
  }
  const next = boundCamera({ ...camera, zoom }, width, height);
  const after = project(next);
  if (!mostlyVisible) {
    const center = after.reduce((s, p) => ({ x: s.x + (p.x * p.weight) / mass, y: s.y + (p.y * p.weight) / mass }), { x: 0, y: 0 });
    next.panX += clamp((rect.left + rect.right) / 2 - center.x, -width * 0.22, width * 0.22);
    next.panY += clamp((rect.top + rect.bottom) / 2 - center.y, -height * 0.18, height * 0.18);
  }
  return {
    camera: boundCamera(next, width, height),
    neighborCount: neighbors.length,
    reason: neighbors.length ? (mostlyVisible ? "already-visible" : "neighbor-bounds") : "isolated",
  };
}

// Restores the same explored world region without rewinding the rotating galaxy.
export function restoreExplorationView(view: { camera: CameraState; rotation?: number }, rotation: number): CameraState {
  const delta = rotation - (view.rotation || 0);
  const center = H.rotateZ({ x: view.camera.tx, y: view.camera.ty, z: view.camera.tz }, delta);
  return { ...view.camera, yaw: view.camera.yaw - delta, tx: center.x, ty: center.y };
}

type NumericCameraKey = "yaw" | "pitch" | "zoom" | "tx" | "ty" | "tz" | "panX" | "panY" | "cx";

export class CameraFlight {
  duration: number;
  active: boolean;
  from!: CameraState;
  to!: CameraState;
  current!: CameraState;
  started = 0;

  constructor(duration = 650) {
    this.duration = duration;
    this.active = false;
  }

  start(from: CameraState, to: CameraState, now: number, reduced = false): CameraState {
    this.from = { ...from };
    this.to = { ...to };
    this.started = now;
    this.active = !reduced;
    this.current = reduced ? { ...to } : { ...from };
    return { ...this.current };
  }

  cancel(): void {
    this.active = false;
  }

  step(now: number): { camera: CameraState; moving: boolean } {
    if (!this.active) return { camera: { ...this.current }, moving: false };
    const t = clamp((now - this.started) / this.duration, 0, 1);
    const ease = t * t * t * (10 + t * (-15 + 6 * t));
    const camera: CameraState = { ...this.from };
    for (const k of Object.keys(this.to) as NumericCameraKey[]) {
      const a = this.from[k];
      const b = this.to[k];
      if (typeof a === "number" && typeof b === "number") {
        camera[k] = a + (b - a) * ease;
      }
    }
    this.active = t < 1;
    this.current = camera;
    return { camera, moving: this.active };
  }
}
