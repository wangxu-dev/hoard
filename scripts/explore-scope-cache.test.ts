import { describe, expect, test } from "bun:test";
import {
  ExploreScopeCache,
  canReuseSnapshot,
  libraryChangeAffectsScope,
} from "../web/src/pages/constellation-dev/scope-cache";

describe("ExploreScopeCache", () => {
  test("evicts least-recently-used scope when exceeding maxEntries and refreshes recency on get", () => {
    const cache = new ExploreScopeCache<{ id: string }, { nodes: number }>(4);
    cache.set(":", { snapshot: { id: "all" }, model: { nodes: 1 }, key: "k0" });
    cache.set("acc-1:douyin", { snapshot: { id: "dy1" }, model: { nodes: 2 }, key: "k1" });
    cache.set("acc-1:bilibili", { snapshot: { id: "b1" }, model: { nodes: 3 }, key: "k2" });
    cache.set("acc-2:", { snapshot: { id: "a2" }, model: { nodes: 4 }, key: "k3" });

    // Touch ":" so "acc-1:douyin" becomes the least-recently-used entry.
    expect(cache.get(":")?.snapshot.id).toBe("all");

    // Adding a 5th scope evicts "acc-1:douyin", while ":" and newer entries stay.
    cache.set(":douyin", { snapshot: { id: "all-dy" }, model: { nodes: 5 }, key: "k4" });
    expect(cache.get("acc-1:douyin")).toBeUndefined();
    expect(cache.get(":")?.snapshot.id).toBe("all");
    expect(cache.get("acc-1:bilibili")?.snapshot.id).toBe("b1");
    expect(cache.get("acc-2:")?.snapshot.id).toBe("a2");
    expect(cache.get(":douyin")?.snapshot.id).toBe("all-dy");

    // Updating an existing key refreshes recency without evicting others.
    expect(cache.get("acc-1:bilibili")?.snapshot.id).toBe("b1");
    cache.set("acc-1:bilibili", { snapshot: { id: "b1-v2" }, model: { nodes: 30 }, key: "k2-v2" });
    cache.set("acc-3:douyin", { snapshot: { id: "dy3" }, model: { nodes: 6 }, key: "k5" });
    // ":" became oldest after acc-1:bilibili, acc-2:, :douyin were accessed/updated.
    expect(cache.get(":")).toBeUndefined();
    expect(cache.get("acc-1:bilibili")?.snapshot.id).toBe("b1-v2");

    cache.clear();
    expect(cache.get("acc-1:bilibili")).toBeUndefined();
    expect(cache.get("acc-3:douyin")).toBeUndefined();
  });

  test("supports multi-scope round-trip hits when switching back and forth", () => {
    const cache = new ExploreScopeCache<{ scope: string }, string>(4);
    const scopes = ["acc-1:douyin", "acc-1:bilibili", ":"];
    for (const scope of scopes) {
      cache.set(scope, { snapshot: { scope }, model: `model:${scope}`, key: `key:${scope}` });
    }
    for (const scope of [...scopes].reverse()) {
      const hit = cache.get(scope);
      expect(hit?.snapshot.scope).toBe(scope);
      expect(hit?.model).toBe(`model:${scope}`);
      expect(hit?.key).toBe(`key:${scope}`);
    }
  });
});

describe("canReuseSnapshot", () => {
  const scope = { accountId: "acc-1", source: "douyin" };
  const cached = {
    schemaVersion: 3,
    inputRevision: "rev-100",
    snapshotId: "snap-1",
    scope: { accountId: "acc-1", source: "douyin" },
  };

  test("reuses snapshot when schemaVersion, scope, inputRevision, and snapshotId match", () => {
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-1" }, cached, scope)).toBe(true);
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "" }, cached, scope)).toBe(true);
    expect(canReuseSnapshot({ inputRevision: "rev-100" }, { ...cached, snapshotId: "" }, scope)).toBe(true);
  });

  test("forces reload when cached is missing, schemaVersion differs, revision changes, scope mismatches, or snapshotId mismatches", () => {
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-1" }, null, scope)).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-1" }, undefined, scope)).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-1" }, { ...cached, schemaVersion: 2 }, scope)).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "", snapshotId: "snap-1" }, cached, scope)).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "rev-101", snapshotId: "snap-1" }, cached, scope)).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-1" }, cached, { accountId: "acc-2", source: "douyin" })).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-1" }, cached, { accountId: "acc-1", source: "bilibili" })).toBe(false);
    expect(canReuseSnapshot({ inputRevision: "rev-100", snapshotId: "snap-2" }, cached, scope)).toBe(false);
  });
});

describe("libraryChangeAffectsScope", () => {
  test("ignores unrelated account changes when account-scoped, and accepts matching or all-account scopes", () => {
    expect(libraryChangeAffectsScope("acc-2", { accountId: "acc-1", source: "" })).toBe(false);
    expect(libraryChangeAffectsScope("acc-2", { accountId: "acc-1", source: "douyin" })).toBe(false);
    expect(libraryChangeAffectsScope("acc-1", { accountId: "acc-1", source: "douyin" })).toBe(true);
    expect(libraryChangeAffectsScope("", { accountId: "acc-1", source: "douyin" })).toBe(true);
    expect(libraryChangeAffectsScope("acc-2", { accountId: "", source: "" })).toBe(true);
    expect(libraryChangeAffectsScope("acc-2", { accountId: "", source: "douyin" })).toBe(true);
  });
});
