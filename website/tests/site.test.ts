import { describe, expect, test } from "bun:test";
import { dictionaries, homePath } from "../lib/content";
import {
  downloadUrl,
  pageMetadata,
  repository,
  siteUrl,
  version,
} from "../lib/site";
import { releaseFiles } from "../../scripts/release-assets";

function keys(value: unknown, prefix = ""): string[] {
  if (!value || typeof value !== "object") return [prefix];
  return Object.entries(value)
    .flatMap(([key, child]) => keys(child, prefix + "." + key))
    .sort();
}

describe("website content and release contract", () => {
  test("both languages have the same copy structure", () => {
    expect(keys(dictionaries.en)).toEqual(keys(dictionaries["zh-CN"]));
    expect(homePath("zh-CN")).toBe("/");
    expect(homePath("en")).toBe("/en/");
  });

  for (const platform of ["darwin", "windows", "linux"] as const) {
    for (const arch of ["arm64", "amd64"] as const) {
      test(
        platform + " " + arch + " uses a published installer filename",
        () => {
          const url = new URL(downloadUrl(platform, arch));
          const filename = url.pathname.split("/").at(-1);
          expect(releaseFiles(version, platform + "-" + arch)).toContain(
            filename,
          );
          expect(url.href).toStartWith(
            repository + "/releases/download/v" + version + "/",
          );
          expect(filename).not.toStartWith("update-");
        },
      );
    }
  }

  test("preview indexing follows the configured origin", () => {
    expect(pageMetadata("zh-CN").robots).toEqual({
      index: Boolean(siteUrl),
      follow: Boolean(siteUrl),
    });
    expect(pageMetadata("en").title).toBe(dictionaries.en.meta.title);
    expect(pageMetadata("en", true).title).toBe(
      dictionaries.en.privacy.metadata,
    );
  });

  test("invalid public origins fail rather than emitting broken canonical URLs", () => {
    const result = Bun.spawnSync(
      [process.execPath, "-e", 'await import("./website/lib/site.ts")'],
      {
        env: {
          ...process.env,
          NEXT_PUBLIC_SITE_URL: "https://example.com/wrong-path",
        },
      },
    );
    expect(result.exitCode).not.toBe(0);
    expect(result.stderr.toString()).toContain("NEXT_PUBLIC_SITE_URL");
  });
});
