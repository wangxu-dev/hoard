import { describe, expect, test } from "bun:test";
import {
  DEFAULT_SOURCE_DESCRIPTORS,
  sourceActionKinds,
  sourceHasFolders,
  sourceLikeIcon,
  sourceUsesFolderPicker,
  visibleListKinds,
} from "../web/src/lib/source-capabilities";
import type { SourceGroup } from "../web/src/lib/library";

const sampleSources: SourceGroup[] = [
  { id: "douyin", name: "Douyin", kinds: ["like", "favorite", "music"], hasFolders: false, accounts: [] },
  { id: "bilibili", name: "Bilibili", kinds: ["favorite", "watch_later"], hasFolders: true, accounts: [] },
];

describe("source capabilities", () => {
  test("single-source scope preserves native list order", () => {
    expect(visibleListKinds(sampleSources, { source: "douyin" })).toEqual(["like", "favorite", "music"]);
    expect(visibleListKinds(sampleSources, { source: "bilibili" })).toEqual(["favorite", "watch_later"]);
  });

  test("all-accounts scope orders merged lists by cross-platform role", () => {
    expect(visibleListKinds(sampleSources, {})).toEqual(["like", "favorite", "watch_later", "music"]);
    // Reversing source order still keeps like -> favorite -> queue -> other stable.
    expect(visibleListKinds([...sampleSources].reverse(), {})).toEqual(["like", "favorite", "watch_later", "music"]);
  });

  test("folder capabilities and action sets follow source descriptors", () => {
    expect(sourceHasFolders("douyin", "favorite")).toBe(false);
    expect(sourceHasFolders("bilibili", "favorite")).toBe(true);
    expect(sourceHasFolders("bilibili", "watch_later")).toBe(false);

    expect(sourceUsesFolderPicker("bilibili", "favorite", "BV1xx")).toBe(true);
    expect(sourceUsesFolderPicker("bilibili", "favorite", "opus:123")).toBe(false);
    expect(sourceUsesFolderPicker("douyin", "favorite", "123")).toBe(false);

    expect(sourceActionKinds("douyin", "video")).toEqual(["like", "favorite"]);
    expect(sourceActionKinds("bilibili", "video")).toEqual(["favorite", "watch_later"]);
    expect(sourceActionKinds("bilibili", "gallery")).toEqual(["favorite"]);
    expect(sourceActionKinds("douyin", "music")).toEqual(["music"]);

    expect(sourceLikeIcon("douyin")).toBe("heart");
    expect(sourceLikeIcon("bilibili")).toBe("thumbs_up");
    expect(DEFAULT_SOURCE_DESCRIPTORS.map((d) => d.id)).toEqual(["douyin", "bilibili"]);
  });
});
