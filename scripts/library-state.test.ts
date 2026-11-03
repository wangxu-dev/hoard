import { beforeEach, expect, test } from "bun:test";
import { libraryScope, reconcileLibraryScope, setLibraryFilters } from "../web/src/lib/library-filters";
import type { SourceGroup } from "../web/src/lib/library";

const sources: SourceGroup[] = [{ id: "douyin", name: "Douyin", kinds: ["like", "favorite", "music"], accounts: [
  { id: "test-a", source: "douyin", nickname: "A", avatarUrl: "", status: "active", syncing: false },
  { id: "test-b", source: "douyin", nickname: "B", avatarUrl: "", status: "active", syncing: false },
] }];
beforeEach(() => setLibraryFilters({ scope: {} }));

test("one account selects its own scope", () => {
  reconcileLibraryScope([{ ...sources[0], accounts: [sources[0].accounts[1]] }]);
  expect(libraryScope()).toEqual({ source: "douyin", accountId: "test-b" });
});

test("multiple accounts preserve explicit all and selected account", () => {
  reconcileLibraryScope(sources);
  expect(libraryScope().accountId).toBeUndefined();
  setLibraryFilters({ scope: { source: "douyin", accountId: "test-b" } });
  reconcileLibraryScope(sources);
  expect(libraryScope().accountId).toBe("test-b");
});

test("removed account falls back without keeping an invalid source", () => {
  setLibraryFilters({ scope: { source: "removed-source", accountId: "removed-account" } });
  reconcileLibraryScope(sources);
  expect(libraryScope().accountId).toBeUndefined();
  expect(libraryScope().source).toBeUndefined();
});
