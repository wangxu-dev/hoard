import type { PlaceRectFn, ProjectedWork, Rect } from "./types";

export interface IdentityEntry {
  p: ProjectedWork;
  rect: Rect;
  selected: boolean;
  related: boolean;
}

export interface IdentityPlanStats {
  projected: number;
  candidates: number;
  edgeFiltered: number;
  communityFiltered: number;
  collisionFiltered: number;
  capFiltered: number;
  drawn: number;
}

export type IdentityPlanResult = IdentityEntry[] & { stats: IdentityPlanStats };

export interface IdentityPlanOptions {
  selectedKey?: string | null;
  selectedNeighbors?: Set<string>;
  width: number;
  height: number;
  zoom: number;
  blocked?: Rect[];
  placeRect: PlaceRectFn;
  cap?: number;
}

// One small cover always represents its real work, never the identity of a community.
export function identityPlan(
  points: ProjectedWork[],
  { selectedKey = null, selectedNeighbors = new Set<string>(), width, height, zoom, blocked = [], placeRect, cap = 9 }: IdentityPlanOptions,
): IdentityPlanResult {
  const near = zoom > 1.65;
  const occupied = blocked.slice();
  const used = new Set<string>();
  const result = [] as unknown as IdentityPlanResult;
  const candidates = points
    .filter((p) => p.x > 24 && p.x < width - 24 && p.y > 85 && p.y < height - 65)
    .slice()
    .sort((a, b) => {
      const rank = (p: ProjectedWork) =>
        p.w.key === selectedKey
          ? 100000
          : selectedNeighbors.has(p.w.key)
            ? 50000
            : Math.min(80, p.w.neighbors?.length || 0) * 20 + (p.depth + 0.8) * 5 + p.w.seed;
      return rank(b) - rank(a) || a.w.key.localeCompare(b.w.key);
    });
  const stats: IdentityPlanStats = {
    projected: points.length,
    candidates: candidates.length,
    edgeFiltered: points.length - candidates.length,
    communityFiltered: 0,
    collisionFiltered: 0,
    capFiltered: 0,
    drawn: 0,
  };
  for (const p of candidates) {
    if (result.length >= cap) {
      stats.capFiltered++;
      continue;
    }
    const selected = p.w.key === selectedKey;
    const groupKey = p.w.community || p.w.key;
    if (!selected && !selectedNeighbors.has(p.w.key) && used.has(groupKey)) {
      stats.communityFiltered++;
      continue;
    }
    const w = selected ? 88 : near ? 72 : 54;
    const h = selected ? 98 : near ? 80 : 60;
    const rect = placeRect(p, w, h, occupied, { x: 14, y: 82, w: width - 28, h: height - 152 }, 12);
    if (!rect) {
      stats.collisionFiltered++;
      continue;
    }
    occupied.push(rect);
    used.add(groupKey);
    result.push({ p, rect, selected, related: selectedNeighbors.has(p.w.key) });
  }
  stats.drawn = result.length;
  result.stats = stats;
  return result;
}
