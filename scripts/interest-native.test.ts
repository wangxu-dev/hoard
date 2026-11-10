import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { analyze, buildScope, cachedAsset, clueMatch } from "./interest-baseline-oracle";
import geometry from "../web/src/pages/constellation-dev/core.js";
import type { LibraryItem } from "../.mygo/bindings.gen";

const item = (id: string, title: string, source = "douyin"): LibraryItem => ({ id, title, source, kind: "video", author: "测试作者", authorId: "", cover: `hoard-cover://localhost/w/${id}`, authorAvatar: `hoard-cover://localhost/a/${id}`, publishedAt: 123, durationMs: 10, imageCount: 0, accountIds: [], likedAccountIds: [], favoritedAccountIds: [], watchLaterAccountIds: [] });
describe("native interest first slice", () => {
 test("token boundaries and unknown/no-text partition", () => {
  expect(clueMatch("classic music", "cs")).toBe(false);
  expect(clueMatch("玩 cs 游戏", "cs")).toBe(true);
  const scope = buildScope([item("1", "摄影旅行"), item("2", "摄影旅行"), item("3", "摄影旅行"), item("4", "无匹配的短语"), item("5", "")], "douyin");
  expect(scope.coverage.worksInTopics).toBe(0);
  expect(scope.coverage.uncategorizedWorks).toBe(4);
  expect(scope.coverage.worksNoText).toBe(1);
  expect(scope.coverage.worksInTopics + scope.coverage.uncategorizedWorks + scope.coverage.worksNoText).toBe(5);
  expect(scope.timeCapabilities.views.source_time).toBe(false);
  expect(scope.works[0].author).not.toHaveProperty("id");
 });
 test("only native cache assets and no fabricated history", () => {
  expect(cachedAsset("https://example.invalid/image")).toBe("");
  expect(cachedAsset("hoard-cover://localhost/w/1")).toEndWith("?cacheOnly=1");
  const w = buildScope([item("1", "摄影")], "douyin").works[0];
  expect(w.monthKeys.source_time).toBeNull();
 });
 test("content-addressed baseline and source-id identity", async () => {
  const a = item("1", "摄影"), b = item("1", "电影", "bilibili");
  const first = await analyze([a, b]), reordered = await analyze([b, a]);
  expect(first.snapshotId).toBe(reordered.snapshotId);
  expect(first.scopes.reduce((n, s) => n + s.works.length, 0)).toBe(2);
  expect((await analyze([{ ...a, title: "不同内容" }, b])).snapshotId).not.toBe(first.snapshotId);
  const abort = new AbortController(); abort.abort();
  expect(analyze([a], abort.signal)).rejects.toThrow("CANCELLED");
 });
 test("real author identities remain source-scoped, never nickname-merged", () => {
  const a = {...item("a", "摄影"), authorId:"author-1"}, b = {...item("b", "电影"), authorId:"author-1"};
  const dy = buildScope([a,b], "douyin"), bili = buildScope([{...a,source:"bilibili"}],"bilibili");
  expect(dy.works[0].author.id).toBe(dy.works[1].author.id);
  expect(dy.works[0].author.id).not.toBe(bili.works[0].author.id);
  expect(dy.coverage.authorIds).toBe(2);
 });
 test("inertia decay is frame-rate independent and zoom anchor holds", () => {
  const sums = [30, 60, 120].map(fps => { let velocity = 2, displacement = 0; for (let i = 0; i < fps; i++) { const s = geometry.inertiaStep(velocity, 1 / fps); velocity = s.velocity; displacement += s.delta; } return displacement; });
  expect(sums[0]).toBeCloseTo(sums[1], 8); expect(sums[1]).toBeCloseTo(sums[2], 8);
  const c = { yaw: .2, pitch: .8, zoom: 1, tx: 0, ty: 0, tz: 0, panX: 0, panY: 0, cx: .5 }, p = { x: .4, y: .5, z: .02 };
  const before = geometry.project(p, c, 1100, 800), after = geometry.project(p, geometry.zoomAround(c, p, 4.7, 1100, 800), 1100, 800);
  expect(before.x).toBeCloseTo(after.x, 8); expect(before.y).toBeCloseTo(after.y, 8);
 });
 test("sampling obeys even tiny caps and is stable under reordering", () => {
  const works=Array.from({length:5000},(_,i)=>({key:`source:${i}`,anchor:`topic-${i%12}`}));
  expect(geometry.sample(works,6)).toHaveLength(6);
  expect(geometry.sample(works,0)).toHaveLength(0);
  const chosen=geometry.sample(works,3000,"source:4999");
  expect(chosen).toHaveLength(3000);expect(new Set(chosen.map(w=>w.key)).size).toBe(3000);
  expect(new Set(chosen.map(w=>w.anchor)).size).toBe(12);expect(chosen.some(w=>w.key==="source:4999")).toBe(true);
  expect(geometry.sample(works,6).map(w=>w.key)).toEqual(geometry.sample([...works].reverse(),6).map(w=>w.key));
 });
 test("native bridge is unconditional; normal landing and startup are preserved", () => {
  const load = readFileSync("web/src/pages/constellation-dev/load.ts", "utf8");
  expect(load).toContain("Exploration.start"); expect(load).toContain("Exploration.result"); expect(load).not.toContain("analyze("); expect(load).not.toContain("Library.list"); expect(load).not.toMatch(/Accounts\.|Sync\.|Player\.|fetch\(/);
  const main = readFileSync("main.go", "utf8"), side = readFileSync("web/src/components/app-sidebar.tsx", "utf8");
  expect(main).toContain('mygo.Bind(service, accounts, syncer, exploration,');
  expect(main).toContain('if service.Store != nil {');expect(main).toContain('app.RestoreAccounts(service)');
  const window = readFileSync("app/window.go", "utf8"), backend = readFileSync("app/exploration.go", "utf8") + readFileSync("app/library.go", "utf8");
  expect(main+window+backend+side).not.toMatch(/HOARD_INTEREST_DEV|dev=constellation|interestDev/);
  expect(window).toContain('URL:       "/"');expect(side).toContain('{ id: "explore" as const, icon: Compass }');
  const renderer = readFileSync("web/src/pages/constellation-dev/renderer.js", "utf8");
  expect(renderer).toContain("orbit.step(now)"); expect(renderer).toContain("selected:false,paused:userPaused"); expect(renderer).toContain("observer.disconnect()");
 });
});
