import { describe, expect, test } from "bun:test";
import { clampRange, matchesPublication, nextPlaybackRange, publicationBins, workMonth } from "../web/src/pages/constellation-dev/publication-time";
import { RelationLayer } from "../web/src/pages/constellation-dev/relation-lines.js";

const dated = (key: string | null) => ({ monthKeys: { source_time: key } });
describe("publication timeline", () => {
  test("fills empty months and keeps unknown dates explicit", () => {
    const result = publicationBins([dated("2024-01"), dated("2024-03"), dated("2024-03"), dated(null), dated("2024-13")]);
    expect(result.bins.map(bin => bin.count)).toEqual([1, 0, 2]);
    expect(result.unknown).toBe(2);
    expect(workMonth(dated("2024-03"))).toBe(2024 * 12 + 2);
    expect(publicationBins([])).toEqual({ bins: [], unknown: 0 });
  });
  test("range includes both endpoints without borrowing another date", () => {
    const jan = 2024 * 12;
    const selection = { range: [jan, jan + 1] as [number, number], includeUnknown: false };
    expect(matchesPublication(dated("2024-01"), selection)).toBe(true);
    expect(matchesPublication(dated("2024-02"), selection)).toBe(true);
    expect(matchesPublication(dated("2024-03"), selection)).toBe(false);
    expect(matchesPublication(dated(null), selection)).toBe(false);
    expect(matchesPublication(dated(null), { ...selection, includeUnknown: true })).toBe(true);
    expect(matchesPublication(dated(null), { range: null, includeUnknown: false })).toBe(true);
  });
  test("whole-window dragging preserves width at both bounds", () => {
    expect(clampRange([-2, 2], 0, 11)).toEqual([0, 4]);
    expect(clampRange([10, 14], 0, 11)).toEqual([7, 11]);
    expect(clampRange([-5, 20], 0, 11)).toEqual([0, 11]);
  });
  test("playback begins at first month, advances a fixed window and restarts at end", () => {
    expect(nextPlaybackRange(null, 0, 11)).toEqual([0, 0]);
    expect(nextPlaybackRange([0, 11], 0, 11)).toEqual([0, 0]);
    expect(nextPlaybackRange([2, 5], 0, 11)).toEqual([3, 6]);
    expect(nextPlaybackRange([8, 11], 0, 11)).toEqual([0, 0]);
  });
  test("out-of-range relation endpoints are excluded without changing graph", () => {
    const a = { key: "a", pos: {}, neighbors: [{ key: "b", weight: .5 }] }, b = { key: "b", pos: {}, neighbors: [{ key: "a", weight: .5 }] };
    const layer = new RelationLayer([a, b], new Map([["a", a], ["b", b]]));
    const frame = layer.frame(a, null);
    const ctx = { globalAlpha: 1, beginPath() {}, moveTo() {}, lineTo() {}, stroke() {} };
    const project = () => ({ x: 10, y: 10, depth: 0 });
    expect(layer.draw(ctx, frame, project, new Set(["a", "b"]), 100, 100, 1, () => false).drawn).toBe(0);
    expect(layer.draw(ctx, frame, project, new Set(["a", "b"]), 100, 100).drawn).toBe(1);
    expect(a.neighbors).toHaveLength(1);
  });
});
