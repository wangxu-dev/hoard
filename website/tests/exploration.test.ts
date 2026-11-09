import { expect, test } from "bun:test";
import { readFileSync, statSync } from "node:fs";
import catalog from "../demo/exploration-catalog.json";
import { createExplorationSnapshot } from "../demo/exploration";

test("public sample contains only display fields and bundled metadata-free thumbnails", () => {
  expect(catalog.items).toHaveLength(300);
  const ids = new Set(catalog.items.map(item => item.id));
  expect(ids.size).toBe(300);
  for (const item of catalog.items) {
    expect(Object.keys(item).sort()).toEqual(["cover", "id", "mediaKind", "source", "title"]);
    expect(item.id).toMatch(/^preview-\d{3}$/);
    expect(item.cover).toBe(`/explore-covers/${item.id}.webp`);
    const file = new URL("../public" + item.cover, import.meta.url);
    expect(statSync(file).size).toBeLessThan(50000);
    expect(readFileSync(file).includes(Buffer.from("EXIF"))).toBe(false);
  }
  for (const edge of catalog.neighbors) {
    expect(ids.has(edge.workId)).toBe(true);
    expect(ids.has(edge.to)).toBe(true);
    expect(edge.shared).toEqual([]);
  }
});

test("each seed changes the presentation while preserving works and actual sample connections", () => {
  const a = createExplorationSnapshot(12), b = createExplorationSnapshot(34);
  expect(createExplorationSnapshot(12)).toEqual(a);
  const works = (snapshot: typeof a) => snapshot.scopes.flatMap(scope => scope.works);
  const titles = (snapshot: typeof a) => works(snapshot).map(work => work.title).sort();
  expect(titles(a)).toEqual(titles(b));
  expect(works(a).map(work => work.id)).not.toEqual(works(b).map(work => work.id));
  expect(works(a).map(work => work.publishedAt)).not.toEqual(works(b).map(work => work.publishedAt));
  const pairs = (snapshot: typeof a) => snapshot.scopes.flatMap(scope => {
    const byId = new Map(scope.works.map(work => [work.id, work.cover]));
    return scope.neighbors.map(edge => [byId.get(edge.workId), byId.get(edge.to)].sort().join("|"));
  }).sort();
  expect(pairs(a)).toEqual(pairs(b));
  expect(pairs(a).length).toBeGreaterThan(100);
  for (const work of works(a)) {
    expect(work.author.name).toMatch(/^创作者 \d{3}$/);
    expect(work.publishedAt).toBeGreaterThanOrEqual(Date.UTC(2020, 0, 1) / 1000);
    expect(work.author.avatarURL).toBe("");
  }
});
