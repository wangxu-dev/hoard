import { graphLayout } from "./graph-layout";
import type { CameraState, ExplorationInput, PreparedModel, PreparedPair, PreparedWork, ProjectedPoint, Rect, Vec3 } from "./types";

/* Independent geometric renderer. No external libraries or reference assets. */
const TAU = Math.PI * 2;

function hash(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  h ^= h >>> 16;
  h = Math.imul(h, 0x85ebca6b);
  h ^= h >>> 13;
  h = Math.imul(h, 0xc2b2ae35);
  return (h ^ (h >>> 16)) >>> 0;
}

function rnd(s: string): number {
  return hash(s) / 4294967296;
}

function normal(s: string): number {
  return Math.sqrt(-2 * Math.log(Math.max(0.00001, rnd(s + "a")))) * Math.cos(TAU * rnd(s + "b"));
}

const spiralWinding = 2.7;

function spiralAngle(radius: number, offset = 0): number {
  return offset + radius * spiralWinding;
}

function rotateZ(p: Vec3, angle: number): Vec3 {
  const c = Math.cos(angle);
  const s = Math.sin(angle);
  return { x: p.x * c - p.y * s, y: p.x * s + p.y * c, z: p.z };
}

// Trailing arms: outward angular winding and material angular velocity have opposite signs.
function materialTurn(delta: number): number {
  return -Math.sign(spiralWinding) * delta;
}

const palette = ["#8edee5", "#c6eee2", "#a6d8f0", "#c9e8dc", "#81d9e1", "#e2e9cd", "#b5d3f1", "#96e2ca", "#e6d8b4", "#96cde3", "#d3e7e3"];

function prepare(data: ExplorationInput): PreparedModel {
  const works: PreparedWork[] = [];
  const byKey = new Map<string, PreparedWork>();
  const pairs = new Map<string, PreparedPair>();
  for (const sc of data.scopes) {
    for (const w of sc.works) {
      const key = sc.source + ":" + w.id;
      if (byKey.has(key)) continue;
      const o: PreparedWork = {
        ...w,
        key,
        source: sc.source,
        seed: rnd(key),
        neighbors: [],
        pos: { x: 0, y: 0, z: 0 },
        community: null,
        color: "#8196aa",
      };
      works.push(o);
      byKey.set(key, o);
    }
  }
  works.sort((a, b) => a.key.localeCompare(b.key));
  for (const sc of data.scopes) {
    for (const n of sc.neighbors || []) {
      const a = byKey.get(sc.source + ":" + n.workId);
      const b = byKey.get(sc.source + ":" + n.to);
      if (!a || !b || a === b || !Number.isFinite(n.score) || n.score <= 0) continue;
      const key = JSON.stringify([a.key, b.key].sort());
      const prior = pairs.get(key);
      if (!prior || n.score > prior.weight) pairs.set(key, { a, b, weight: n.score });
    }
  }
  // Stable edge iteration also makes cold-cache/input-order results identical.
  const sortedPairs = new Map([...pairs].sort((a, b) => a[0].localeCompare(b[0])));
  for (const { a, b, weight } of sortedPairs.values()) {
    a.neighbors.push({ key: b.key, weight });
    b.neighbors.push({ key: a.key, weight });
  }
  for (const w of works) w.neighbors.sort((x, y) => y.weight - x.weight || x.key.localeCompare(y.key));
  const layout = graphLayout(works, sortedPairs, { rnd, normal, hash, palette });
  const layoutFingerprint = works.reduce(
    (v, w) => Math.imul(v ^ hash(w.key + ":" + [w.pos.x, w.pos.y, w.pos.z].map((x) => x.toFixed(6)).join(",")), 16777619) >>> 0,
    2166136261,
  );
  return {
    works,
    byKey,
    ...layout,
    stats: { ...layout.stats, total: works.length, edges: sortedPairs.size },
    layoutFingerprint,
    allKeys: new Set(works.map((w) => w.key)),
    initialStars: sample(works),
  };
}

function sample(works: PreparedWork[], cap = 6500, selectedKey: string | null = null): PreparedWork[] {
  cap = Math.max(0, Math.floor(cap));
  if (!cap) return [];
  if (works.length <= cap) return works.slice();
  const rank = (a: PreparedWork, b: PreparedWork) => hash(a.key) - hash(b.key) || a.key.localeCompare(b.key);
  const linked = works.filter((w) => w.neighbors?.length);
  const isolates = works.filter((w) => !w.neighbors?.length);
  // Keep visible pairs/community cores; purely random thinning was erasing actual structure.
  const groups = new Map<string, PreparedWork[]>();
  for (const w of linked) {
    const c = w.community || w.key;
    if (!groups.has(c)) groups.set(c, []);
    groups.get(c)!.push(w);
  }
  const picked: PreparedWork[] = [];
  const used = new Set<string>();
  const add = (w: PreparedWork | undefined) => {
    if (w && !used.has(w.key) && picked.length < cap) {
      picked.push(w);
      used.add(w.key);
    }
  };
  if (selectedKey) {
    const w = works.find((item) => item.key === selectedKey);
    add(w);
    const by = new Map(works.map((item) => [item.key, item]));
    for (const n of (w?.neighbors || []).slice(0, 120)) add(by.get(n.key));
  }
  const linkedCap = Math.min(linked.length, Math.floor(cap * 0.7));
  for (const a of [...groups.values()].sort((a, b) => (a[0].community || "").localeCompare(b[0].community || "") || rank(a[0], b[0]))) {
    a.sort((x, y) => y.neighbors.length - x.neighbors.length || rank(x, y));
    for (const w of a.slice(0, 2)) {
      if (picked.length >= linkedCap) break;
      add(w);
    }
  }
  for (const w of linked.sort(rank)) {
    if (picked.length >= linkedCap) break;
    add(w);
  }
  for (const w of isolates.sort(rank)) add(w);
  for (const w of works.slice().sort(rank)) add(w);
  return picked;
}

function transform(pos: Vec3, camera: CameraState): Vec3 {
  const x = pos.x - camera.tx;
  const y = pos.y - camera.ty;
  const z = pos.z - camera.tz;
  const ca = Math.cos(camera.yaw);
  const sa = Math.sin(camera.yaw);
  const cp = Math.cos(camera.pitch);
  const sp = Math.sin(camera.pitch);
  const rx = x * ca - y * sa;
  const ry = x * sa + y * ca;
  return { x: rx, y: ry * cp - z * sp, z: ry * sp + z * cp };
}

function project(pos: Vec3, camera: CameraState, width: number, height: number): ProjectedPoint {
  const q = transform(pos, camera);
  const p = 3.6 / Math.max(0.65, 3.6 - q.z);
  const scale = Math.min(width * 0.375, height * 0.59) * camera.zoom;
  return { x: width * (camera.cx || 0.55) + q.x * scale * p + camera.panX, y: height * 0.515 + q.y * scale * p + camera.panY, depth: q.z, p, scale };
}

function pick<T extends { x: number; y: number }>(buffer: T[], x: number, y: number, max = 10): T | null {
  let best: T | null = null;
  let dist = max;
  for (const p of buffer) {
    const d = Math.hypot(p.x - x, p.y - y);
    if (d < dist) {
      dist = d;
      best = p;
    }
  }
  return best;
}

function mst(stars: PreparedWork[], limit = 38): [PreparedWork, PreparedWork][] {
  // A sparse Euclidean spanning tree for visual organisation, NOT behavioural inference.
  const a = stars.slice().sort((x, y) => hash(x.key) - hash(y.key)).slice(0, limit);
  const edges: [PreparedWork, PreparedWork][] = [];
  if (a.length < 2) return edges;
  const used = new Set([0]);
  while (used.size < a.length) {
    let best: { i: number; j: number; d: number } | null = null;
    for (const i of used) {
      for (let j = 0; j < a.length; j++) {
        if (!used.has(j)) {
          const p = a[i].pos;
          const q = a[j].pos;
          const d = (p.x - q.x) ** 2 + (p.y - q.y) ** 2 + (p.z - q.z) ** 2;
          if (!best || d < best.d) best = { i, j, d };
        }
      }
    }
    if (!best) break;
    used.add(best.j);
    edges.push([a[best.i], a[best.j]]);
  }
  return edges;
}

function overlapRect(a: Rect, b: Rect, pad = 0): boolean {
  return a.x < b.x + b.w + pad && a.x + a.w + pad > b.x && a.y < b.y + b.h + pad && a.y + a.h + pad > b.y;
}

function placeRect(point: { x: number; y: number }, width: number, height: number, occupied: Rect[], bounds: Rect, gap = 8): Rect | null {
  const offsets: [number, number][] = [
    [15, -height - 12],
    [15, 16],
    [-width - 16, -height - 12],
    [-width - 16, 16],
    [18, -height - 52],
    [35, 48],
    [-width - 35, 48],
    [18, -height - 94],
    [-width - 18, -height - 94],
  ];
  for (const [dx, dy] of offsets) {
    const r: Rect = { x: point.x + dx, y: point.y + dy, w: width, h: height };
    if (r.x < bounds.x || r.y < bounds.y || r.x + r.w > bounds.x + bounds.w || r.y + r.h > bounds.y + bounds.h) continue;
    if (!occupied.some((o) => overlapRect(r, o, gap))) return r;
  }
  return null;
}

function zoomAround(
  camera: CameraState,
  world: Vec3 | null,
  nextZoom: number,
  width: number,
  height: number,
  desired: { x: number; y: number } | null = null,
): CameraState {
  const next: CameraState = { ...camera, zoom: Math.max(0.55, Math.min(world ? 9 : 1.8, nextZoom)) };
  if (world) {
    const before = desired || project(world, camera, width, height);
    const after = project(world, next, width, height);
    next.panX += before.x - after.x;
    next.panY += before.y - after.y;
  }
  return next;
}

function unprojectPlane(camera: CameraState, x: number, y: number, width: number, height: number): Vec3 {
  const scale = Math.min(width * 0.375, height * 0.59) * camera.zoom;
  const sx = (x - width * (camera.cx || 0.55) - camera.panX) / scale;
  const sy = (y - height * 0.515 - camera.panY) / scale;
  const cp = Math.cos(camera.pitch);
  const sp = Math.sin(camera.pitch);
  const z = -camera.tz;
  const ry = (3.6 * sy - sy * z * cp + 3.6 * z * sp) / (3.6 * cp + sy * sp);
  const rx = (sx * (3.6 - ry * sp - z * cp)) / 3.6;
  const ca = Math.cos(camera.yaw);
  const sa = Math.sin(camera.yaw);
  return { x: rx * ca + ry * sa + camera.tx, y: -rx * sa + ry * ca + camera.ty, z: 0 };
}

function wheelFactor(deltaY: number, deltaMode = 0, ctrlKey = false): number {
  const pixels = deltaY * (deltaMode === 1 ? 16 : deltaMode === 2 ? 600 : 1);
  return Math.exp(Math.max(-0.65, Math.min(0.65, -pixels * (ctrlKey ? 0.004 : 0.0015))));
}

type NumericCameraKey = "yaw" | "pitch" | "zoom" | "tx" | "ty" | "tz" | "panX" | "panY" | "cx";

function cameraStep(camera: CameraState, target: CameraState, dt: number, reduced = false): { camera: CameraState; moving: boolean } {
  const a = reduced ? 1 : 1 - Math.exp(-10 * Math.min(0.05, dt));
  const next: CameraState = { ...camera };
  let moving = false;
  for (const k of Object.keys(target) as NumericCameraKey[]) {
    const tVal = target[k];
    const cVal = camera[k];
    if (typeof tVal !== "number" || typeof cVal !== "number") continue;
    const d = tVal - cVal;
    next[k] = Math.abs(d) < 0.00002 ? tVal : cVal + d * a;
    if (Math.abs(d) > 0.00002) moving = true;
  }
  return { camera: next, moving };
}

function inertiaStep(velocity: number, dt: number, decay = 6.4): { delta: number; velocity: number } {
  const t = Math.max(0, Math.min(0.05, dt));
  const d = Math.exp(-decay * t);
  return { delta: (velocity * (1 - d)) / decay, velocity: velocity * d };
}

const api = {
  spiralWinding,
  spiralAngle,
  rotateZ,
  materialTurn,
  hash,
  rnd,
  normal,
  prepare,
  sample,
  project,
  pick,
  mst,
  palette,
  overlapRect,
  placeRect,
  zoomAround,
  inertiaStep,
  wheelFactor,
  cameraStep,
  unprojectPlane,
};

export default api;
