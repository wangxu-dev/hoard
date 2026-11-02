import { describe, expect, test } from "bun:test";
import { releaseNotes, validateRelease } from "./release-notes";

describe("release validation", () => {
  const notes = "# Changes\n\n## 1.2.0\n\n- Current\n\n## 1.1.0\n\n- Previous\n";
  test("extracts only the matching version", () => {
    expect(validateRelease("v1.2.0", "1.2.0", notes)).toBe("- Current");
    expect(releaseNotes("## [1.2.0] - 2026-11-02\n\n- Fix", "1.2.0")).toBe("- Fix");
  });
  test("rejects mismatch, missing notes and invalid versions", () => {
    expect(() => validateRelease("v1.1.0", "1.2.0", notes)).toThrow();
    expect(() => validateRelease("v1.3.0", "1.3.0", notes)).toThrow();
    expect(() => validateRelease("v01.2.0", "01.2.0", notes)).toThrow();
    expect(() => releaseNotes("## 1.2.0", "1.2.0")).toThrow();
    expect(() => releaseNotes("## 1.2.0\n", "1.2.0")).toThrow();
  });
});
