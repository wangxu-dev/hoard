import assert from "node:assert/strict";
import H from "../web/src/pages/constellation-dev/core.ts";
import { RelationLayer } from "../web/src/pages/constellation-dev/relation-lines.ts";

const input = {
  scopes: [{
    source: "douyin",
    works: [{ id: "item-a" }, { id: "item-b" }, { id: "item-c" }],
    neighbors: [{ workId: "item-a", to: "item-b", score: 0.8 }],
  }],
};
const prepared = H.prepare(input);
prepared.initialRelations = new RelationLayer(prepared.works, prepared.byKey).background;
const restored = structuredClone(prepared);

assert.equal(prepared.stats.total, 3);
assert.equal(prepared.stats.edges, 1);
assert.equal(prepared.layoutFingerprint, H.prepare(input).layoutFingerprint);
assert.deepEqual(prepared.initialStars.map((work) => work.key), H.sample(prepared.works).map((work) => work.key));
assert.deepEqual([...prepared.allKeys].sort(), prepared.works.map((work) => work.key).sort());
assert.equal(restored.initialStars[0], restored.byKey.get(restored.initialStars[0].key));
assert.deepEqual(
  new RelationLayer(restored.works, restored.byKey, 42, restored.initialRelations).background.map(({ a, b, weight }) => [a.key, b.key, weight]),
  new RelationLayer(restored.works, restored.byKey).background.map(({ a, b, weight }) => [a.key, b.key, weight]),
);

console.log("exploration preparation passed");
