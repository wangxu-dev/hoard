import { describe, expect, test } from "bun:test";
import { mediaScopeAccount, nextTrackSrc } from "../web/src/lib/media-key";

describe("music playback cache stability", () => {
  test("music media scope ignores library account changes while video stays scoped", () => {
    expect(mediaScopeAccount("music", "acc-1")).toBe("");
    expect(mediaScopeAccount("music", "acc-2")).toBe("");
    expect(mediaScopeAccount("music", undefined)).toBe("");
    expect(mediaScopeAccount("video", "acc-1")).toBe("acc-1");
    expect(mediaScopeAccount("gallery", "acc-2")).toBe("acc-2");
  });

  test("active track URL stays locked across background refetches and updates on switch or error recovery", () => {
    const initial = nextTrackSrc(null, "song-1", "http://127.0.0.1:9/video/a", false);
    expect(initial).toEqual({ id: "song-1", url: "http://127.0.0.1:9/video/a" });

    // Background refetch returning a different URL must not replace the playing URL.
    const sameTrack = nextTrackSrc(initial, "song-1", "http://127.0.0.1:9/video/b", false);
    expect(sameTrack).toBe(initial);

    // Explicit error recovery replaces the URL for the same track.
    const recovered = nextTrackSrc(initial, "song-1", "http://127.0.0.1:9/video/b", true);
    expect(recovered).toEqual({ id: "song-1", url: "http://127.0.0.1:9/video/b" });

    // Switching to another track immediately updates the URL.
    const switched = nextTrackSrc(recovered, "song-2", "http://127.0.0.1:9/video/c", false);
    expect(switched).toEqual({ id: "song-2", url: "http://127.0.0.1:9/video/c" });
  });
});
