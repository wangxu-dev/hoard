import type { PlaceRectFn, PreparedWork, ProjectedWork, Rect } from "./types";

export interface CoverDirectorItem {
  p: ProjectedWork;
  dx: number;
  dy: number;
  w: number;
  h: number;
  phase: "pending" | "enter" | "visible" | "exit";
  opacity: number;
  born: number;
  fadeAt: number;
  exitFrom?: number;
  replace: string | null;
}

export interface CoverDirectorStats {
  admitted: number;
  retired: number;
  handoffs: number;
  distinct: number;
  midFade: number;
  candidates?: number;
  projected?: number;
}

export interface CoverDirectorUpdateOptions {
  now: number;
  width: number;
  height: number;
  zoom: number;
  blocked?: Rect[];
  placeRect: PlaceRectFn;
  cap?: number;
  selectedKey?: string | null;
  hoverKey?: string | null;
  advancing?: boolean;
  motion?: number;
  reduced?: boolean;
  ready?: (w: PreparedWork) => boolean;
  failed?: (w: PreparedWork) => boolean | undefined;
}

// Display leases affect only which real work covers are visible. Never positions or relations.
const exposure = new Map<string, number>();
let sessions = 0;

function hash(s: string): number {
  let x = 2166136261;
  for (let i = 0; i < s.length; i++) {
    x ^= s.charCodeAt(i);
    x = Math.imul(x, 16777619);
  }
  return (x >>> 0) / 4294967296;
}

function remember(key: string, dt: number): void {
  const n = (exposure.get(key) || 0) + dt;
  exposure.delete(key);
  exposure.set(key, n);
  if (exposure.size > 8192) {
    const oldest = exposure.keys().next().value;
    if (oldest !== undefined) exposure.delete(oldest);
  }
}

export class CoverDirector {
  seed: string;
  hold: number;
  fade: number;
  interval: number;
  items: Map<string, CoverDirectorItem>;
  lastAdmission: number;
  lastTick: number | null;
  lastMotion: number | null;
  motionCredit: number;
  failed: Set<string>;
  stats: CoverDirectorStats;
  seen: Set<string>;

  constructor({ seed, hold = 6500, fade = 1100, interval = 1600 }: { seed?: string; hold?: number; fade?: number; interval?: number } = {}) {
    this.seed = seed ?? `${++sessions}:${globalThis.crypto?.getRandomValues(new Uint32Array(1))[0] ?? Date.now()}`;
    this.hold = hold;
    this.fade = fade;
    this.interval = interval;
    this.items = new Map();
    this.lastAdmission = -Infinity;
    this.lastTick = null;
    this.lastMotion = null;
    this.motionCredit = 0;
    this.failed = new Set();
    this.stats = { admitted: 0, retired: 0, handoffs: 0, distinct: 0, midFade: 0 };
    this.seen = new Set();
  }

  score(p: ProjectedWork, width: number, height: number): number {
    const x = p.x / width;
    const y = p.y / height;
    const edge = Math.max(0, Math.min(x, 1 - x, y, 1 - y) * 4);
    const depth = Math.max(0, Math.min(1, (p.depth + 1.3) / 2.6));
    return (
      hash(this.seed + ":" + p.w.key) * 0.8 +
      edge * 0.28 +
      depth * 0.26 +
      Math.min(0.12, (p.w.neighbors?.length || 0) * 0.012) -
      Math.min(1.4, (exposure.get(p.w.key) || 0) / 14000)
    );
  }

  tick(now: number): boolean {
    const dt = this.lastTick === null ? 0 : Math.max(0, Math.min(80, now - this.lastTick));
    this.lastTick = now;
    let moving = false;
    let mid = 0;
    for (const [key, r] of this.items) {
      if (r.phase === "pending") {
        moving = true;
      } else if (r.phase === "enter") {
        r.opacity = Math.min(1, (now - r.fadeAt) / this.fade);
        if (r.opacity >= 1) r.phase = "visible";
        else moving = true;
      } else if (r.phase === "exit") {
        r.opacity = Math.max(0, (r.exitFrom ?? 1) * (1 - (now - r.fadeAt) / this.fade));
        if (r.opacity <= 0) {
          this.items.delete(key);
          this.stats.retired++;
          continue;
        }
        moving = true;
      }
      if (r.opacity > 0) {
        remember(key, dt * r.opacity);
        if (r.opacity < 0.98) mid++;
      }
    }
    this.stats.midFade = mid;
    return moving;
  }

  update(
    points: ProjectedWork[],
    {
      now,
      width,
      height,
      zoom,
      blocked = [],
      placeRect,
      cap = 13,
      selectedKey = null,
      hoverKey = null,
      advancing = false,
      motion = 0,
      reduced = false,
      ready = () => false,
      failed = () => false,
    }: CoverDirectorUpdateOptions,
  ): void {
    const byKey = new Map(points.map((p) => [p.w.key, p]));
    const locked = new Set([selectedKey, hoverKey].filter((k): k is string => Boolean(k)));
    if (this.lastMotion !== null && advancing) this.motionCredit += Math.abs(motion - this.lastMotion);
    this.lastMotion = motion;
    for (const [key, r] of this.items) {
      const p = byKey.get(key);
      if (p) r.p = p;
      if (locked.has(key) && r.phase === "exit") {
        r.phase = "visible";
        r.opacity = 1;
        r.replace = null;
      }
    }
    // Wait for the exact new work's cache-only image before fading out its predecessor.
    for (const [key, r] of this.items) {
      if (r.phase !== "pending") continue;
      if (failed(r.p.w)) {
        this.failed.add(key);
        this.items.delete(key);
        continue;
      }
      if (ready(r.p.w)) {
        const victim = r.replace ? this.items.get(r.replace) : undefined;
        if (victim && r.replace && locked.has(r.replace)) {
          this.items.delete(key);
          continue;
        }
        if (victim) {
          victim.phase = "exit";
          victim.fadeAt = now;
          victim.exitFrom = victim.opacity;
          this.stats.handoffs++;
        }
        r.phase = reduced ? "visible" : "enter";
        r.fadeAt = now;
        r.opacity = reduced ? 1 : 0;
        r.born = now;
        this.stats.admitted++;
        this.seen.add(key);
        this.stats.distinct = this.seen.size;
      }
    }
    const cell = (p: ProjectedWork) => Math.max(0, Math.min(3, Math.floor((p.x / width) * 4))) + 4 * Math.max(0, Math.min(2, Math.floor((p.y / height) * 3)));
    const cells = new Map<number, number>();
    const occupied = blocked.slice();
    const groups = new Set<string>();
    for (const r of this.items.values()) {
      const c = cell(r.p);
      cells.set(c, (cells.get(c) || 0) + 1);
      occupied.push({ x: r.p.x + r.dx, y: r.p.y + r.dy, w: r.w, h: r.h });
      groups.add(r.p.w.community || r.p.w.key);
    }
    const valid = points.filter((p) => p.x > 24 && p.x < width - 24 && p.y > 85 && p.y < height - 65 && !this.items.has(p.w.key) && !this.failed.has(p.w.key));
    valid.sort((a, b) => (cells.has(cell(b)) ? 0 : 10) + this.score(b, width, height) - ((cells.has(cell(a)) ? 0 : 10) + this.score(a, width, height)) || a.w.key.localeCompare(b.w.key));
    this.stats.candidates = valid.length;
    this.stats.projected = points.length;
    const pending = [...this.items.values()].some((r) => r.phase === "pending" || r.phase === "exit");
    const live = [...this.items.values()].filter((r) => r.phase !== "exit");
    const forced = selectedKey && !this.items.has(selectedKey) ? byKey.get(selectedKey) ?? null : null;
    const initial = live.length < cap;
    if (!forced && (pending || now - this.lastAdmission < (initial ? 140 : this.interval) || (!initial && (!advancing || this.motionCredit < 0.025)))) return;
    let victim: CoverDirectorItem | null = null;
    if (!initial && !forced) {
      victim =
        live
          .filter(
            (r) =>
              !locked.has(r.p.w.key) &&
              r.phase === "visible" &&
              (now - r.born >= this.hold || ((r.p.x < 0 || r.p.x > width || r.p.y < 0 || r.p.y > height) && now - r.born > 1200)),
          )
          .sort((a, b) => (cells.get(cell(b.p)) || 0) - (cells.get(cell(a.p)) || 0) || this.score(a.p, width, height) - this.score(b.p, width, height))[0] ?? null;
      if (!victim) return;
    }
    for (const p of forced ? [forced] : valid) {
      if (!forced && groups.has(p.w.community || p.w.key)) continue;
      if (victim && (cells.get(cell(victim.p)) || 0) <= 1 && cell(p) !== cell(victim.p) && cells.has(cell(p))) continue;
      if (victim && this.score(p, width, height) < this.score(victim.p, width, height) + 0.12 && cells.has(cell(p))) continue;
      const selected = p.w.key === selectedKey;
      const w = selected ? 88 : zoom > 1.65 ? 72 : 54;
      const h = selected ? 98 : zoom > 1.65 ? 80 : 60;
      const rect = placeRect(p, w, h, occupied, { x: 14, y: 82, w: width - 28, h: height - 152 }, 12);
      if (!rect) continue;
      if (forced && !initial) {
        victim =
          live
            .filter((r) => !locked.has(r.p.w.key) && r.phase === "visible")
            .sort((a, b) => this.score(a.p, width, height) - this.score(b.p, width, height))[0] ?? null;
      }
      this.items.set(p.w.key, {
        p,
        dx: rect.x - p.x,
        dy: rect.y - p.y,
        w,
        h,
        phase: "pending",
        opacity: 0,
        born: now,
        fadeAt: now,
        replace: victim?.p.w.key || null,
      });
      this.lastAdmission = now;
      this.motionCredit = 0;
      ready(p.w);
      break;
    }
  }

  snapshot() {
    return {
      ...this.stats,
      count: this.items.size,
      visible: [...this.items.values()].filter((r) => r.opacity > 0.02).length,
      pending: [...this.items.values()].filter((r) => r.phase === "pending").length,
      entering: [...this.items.values()].filter((r) => r.phase === "enter").length,
      exiting: [...this.items.values()].filter((r) => r.phase === "exit").length,
    };
  }
}
