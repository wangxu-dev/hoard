import H from "./core";
import type { BackgroundEdge, PreparedWork, ProjectedPoint, Vec3 } from "./types";

export interface RelationFrameEdge {
  a: PreparedWork;
  b: PreparedWork;
  weight: number;
  local: boolean;
}

export interface RelationFrame {
  edges: RelationFrameEdge[];
  weights: Map<string, number>;
}

export interface RelationDrawStats {
  drawn: number;
  local: number;
  weak: number;
  budget: number;
}

const strength = (w: number): number => Math.sqrt(Math.max(0, Math.min(1, w)));

export function relationStyle(weight: number, local = false): { alpha: number; width: number } {
  const s = strength(weight);
  return { alpha: local ? 0.19 + 0.53 * s : 0.035 + 0.075 * s, width: local ? 0.45 + 1.35 * s : 0.35 + 0.45 * s };
}

// O(E) at mount, bounded O(budget+local degree) per frame. No spatial inference.
export class RelationLayer {
  byKey: Map<string, PreparedWork>;
  background: BackgroundEdge[];

  constructor(works: PreparedWork[], byKey: Map<string, PreparedWork>, budget = 42, background: BackgroundEdge[] | null = null) {
    this.byKey = byKey;
    if (background) {
      this.background = background;
      return;
    }
    const slots: (BackgroundEdge | undefined)[] = new Array(budget);
    for (const a of works) {
      for (const n of a.neighbors || []) {
        const b = byKey.get(n.key);
        if (!b || a.key >= b.key || !(n.weight > 0)) continue;
        const rank = H.hash(a.key + "|" + b.key);
        const slot = rank % budget;
        if (!slots[slot] || rank < slots[slot]!.rank) slots[slot] = { a, b, weight: n.weight, rank };
      }
    }
    this.background = slots.filter((e): e is BackgroundEdge => Boolean(e));
  }

  frame(selected: PreparedWork | null | undefined, hover: PreparedWork | null | undefined): RelationFrame {
    const edges = new Map<string, RelationFrameEdge>();
    const weights = new Map<string, number>();
    const add = (a: PreparedWork, b: PreparedWork, weight: number, local: boolean) => {
      const key = [a.key, b.key].sort().join("|");
      const prior = edges.get(key);
      if (!prior || local) edges.set(key, { a, b, weight, local });
    };
    for (const e of this.background) add(e.a, e.b, e.weight, false);
    const sources: [PreparedWork | null | undefined, number][] = [
      [selected, 36],
      [hover, 24],
    ];
    for (const [work, cap] of sources) {
      if (!work) continue;
      weights.set(work.key, 1);
      for (const n of (work.neighbors || []).slice(0, cap)) {
        const b = this.byKey.get(n.key);
        if (!b || !(n.weight > 0)) continue;
        weights.set(b.key, Math.max(weights.get(b.key) || 0, n.weight));
        add(work, b, n.weight, true);
      }
    }
    return { edges: [...edges.values()], weights };
  }

  draw(
    ctx: CanvasRenderingContext2D,
    frame: RelationFrame,
    project: (p: Vec3) => ProjectedPoint,
    rendered: Set<string>,
    width: number,
    height: number,
    opacity = 1,
    matches: (work: PreparedWork) => boolean = () => true,
  ): RelationDrawStats {
    let drawn = 0;
    let local = 0;
    let weak = 0;
    for (const e of frame.edges) {
      if (!matches(e.a) || !matches(e.b)) continue;
      if (!rendered.has(e.a.key) || !rendered.has(e.b.key)) continue;
      const a = project(e.a.pos);
      const b = project(e.b.pos);
      if ((a.x < 0 && b.x < 0) || (a.y < 0 && b.y < 0) || (a.x > width && b.x > width) || (a.y > height && b.y > height)) continue;
      const style = relationStyle(e.weight, e.local);
      const depth = Math.max(0.25, Math.min(1, (Math.min(a.depth, b.depth) + 1.6) / 2.4));
      ctx.globalAlpha = style.alpha * depth * opacity;
      ctx.strokeStyle = e.local ? "#a2e4df" : "#659ba8";
      ctx.lineWidth = style.width;
      ctx.beginPath();
      ctx.moveTo(a.x, a.y);
      ctx.lineTo(b.x, b.y);
      ctx.stroke();
      drawn++;
      if (e.local) local++;
      if (e.weight < 0.25) weak++;
    }
    ctx.globalAlpha = 1;
    return { drawn, local, weak, budget: this.background.length + 60 };
  }
}
