import { describe, expect, test } from "bun:test";
import { resolveVideoQuality, type VideoQualityOption } from "../web/src/lib/video-quality";

const sampleQualities: VideoQualityOption[] = [
  { id: "2160", label: "4K", videos: ["http://127.0.0.1:9000/v/4k", "http://127.0.0.1:9000/v/shared"] },
  { id: "1080", label: "1080P", videos: ["http://127.0.0.1:9000/v/1080"] },
  { id: "720", label: "720P", videos: ["http://127.0.0.1:9000/v/720"] },
];

const fallbacks = [
  "http://127.0.0.1:9000/v/shared",
  "http://127.0.0.1:9000/v/1080",
  "http://127.0.0.1:9000/v/play",
  "http://127.0.0.1:9000/v/play",
];

describe("resolveVideoQuality", () => {
  test("auto and empty preferred select highest tier", () => {
    expect(resolveVideoQuality(sampleQualities, fallbacks, "auto")).toEqual({
      activeId: "2160",
      activeLabel: "4K",
      videos: [
        "http://127.0.0.1:9000/v/4k",
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/play",
      ],
    });
    expect(resolveVideoQuality(sampleQualities, fallbacks, "")).toEqual({
      activeId: "2160",
      activeLabel: "4K",
      videos: [
        "http://127.0.0.1:9000/v/4k",
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/play",
      ],
    });
  });

  test("exact match selects requested tier and deduplicates fallbacks", () => {
    expect(resolveVideoQuality(sampleQualities, fallbacks, "1080")).toEqual({
      activeId: "1080",
      activeLabel: "1080P",
      videos: [
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/play",
      ],
    });
  });

  test("falls back to highest tier <= preferred when exact match missing", () => {
    expect(resolveVideoQuality(sampleQualities, fallbacks, "1440")).toEqual({
      activeId: "1080",
      activeLabel: "1080P",
      videos: [
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/play",
      ],
    });
  });

  test("falls back to lowest available tier when all tiers are higher than preferred", () => {
    expect(resolveVideoQuality(sampleQualities, fallbacks, "480")).toEqual({
      activeId: "720",
      activeLabel: "720P",
      videos: [
        "http://127.0.0.1:9000/v/720",
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/play",
      ],
    });
  });

  test("handles empty or undefined qualities using deduplicated fallback videos", () => {
    expect(resolveVideoQuality(undefined, fallbacks, "1080")).toEqual({
      activeId: "",
      activeLabel: "",
      videos: [
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/play",
      ],
    });
    expect(resolveVideoQuality([], fallbacks, "auto")).toEqual({
      activeId: "",
      activeLabel: "",
      videos: [
        "http://127.0.0.1:9000/v/shared",
        "http://127.0.0.1:9000/v/1080",
        "http://127.0.0.1:9000/v/play",
      ],
    });
  });
});
