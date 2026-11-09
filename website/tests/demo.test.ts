import { describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { Accounts, Exploration, Library, Player, Sync, demoEntries } from "../demo/backend";

describe("website demo data", () => {
  test("the library-only demo rejects native exploration explicitly", async () => {
    await expect(Exploration.start()).rejects.toThrow("EXPLORATION_UNAVAILABLE_IN_DEMO");
    await expect(Exploration.cancel()).resolves.toBeUndefined();
  });
  test("shared player and automatic sync APIs remain available", async () => {
    const item = demoEntries[0];
    const result = await Player.resolve(item.kind, item.id, item.accountIds[0], false, "");
    expect(result.images).toEqual([{ url: item.cover }]);
    await expect(Sync.autoStart()).resolves.toBeUndefined();
  });
  test("all fifty works have distinct local covers", () => {
    expect(demoEntries).toHaveLength(50);
    expect(new Set(demoEntries.map(item => item.id)).size).toBe(50);
    const hashes = demoEntries.map(item => {
      expect(item.cover).toStartWith("/covers/");
      return createHash("sha256").update(readFileSync(new URL("../public" + item.cover, import.meta.url))).digest("hex");
    });
    expect(new Set(hashes).size).toBe(50);
  });
  test("pagination, search and content filters describe the same works", async () => {
    const first = await Library.list({ offset: 0, limit: 20 });
    const next = await Library.list({ offset: 20, limit: 20 });
    expect(first.total).toBe(50);
    expect(new Set([...first.items, ...next.items].map(item => item.id)).size).toBe(40);
    const search = await Library.list({ query: demoEntries[0].title });
    expect(search.items).toEqual([demoEntries[0]]);
    const videos = await Library.list({ type: "video" });
    expect(videos.total).toBeGreaterThan(0);
    expect(videos.items.every(item => item.kind === "video")).toBe(true);
  });
  test("timeline offsets match the filtered list", async () => {
    const q = { source: "bilibili", type: "gallery", sort: "published" };
    const list = await Library.list(q), months = await Library.timeline(q);
    expect(months.reduce((sum, month) => sum + month.count, 0)).toBe(list.total);
    for (const month of months) {
      const date = new Date(list.items[month.start].publishedAt * 1000);
      expect([date.getFullYear(), date.getMonth() + 1]).toEqual([month.year, month.month]);
    }
  });
  test("example accounts have names and local avatars", async () => {
    const accounts = await Accounts.list();
    expect(accounts).toHaveLength(2);
    for (const entry of demoEntries) {
      expect(accounts.every(account => account.avatar !== entry.authorAvatar)).toBe(true);
      expect(accounts.every(account => !account.nickname.includes(entry.author))).toBe(true);
    }
    for (const account of accounts) {
      expect(account.nickname.length).toBeGreaterThan(0);
      expect(account.avatar).toStartWith("/demo-avatars/");
      expect((await Library.list({ accountId: account.id })).total).toBe(25);
    }
  });
});
