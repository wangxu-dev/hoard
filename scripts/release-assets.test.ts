import { expect, test } from "bun:test";
import { releaseFiles, releaseTargets } from "./release-assets";

test("six architectures produce distinct release assets", () => {
  const names = releaseTargets.flatMap((target) => releaseFiles("0.1.0", target));
  expect(names).toHaveLength(18);
  expect(new Set(names).size).toBe(18);
  expect(releaseFiles("0.1.0", "windows-arm64")).toContain("hoard-setup-0.1.0-arm64.exe");
  expect(releaseFiles("0.1.0", "darwin-amd64")).toContain("hoard-0.1.0-amd64.dmg");
  expect(() => releaseFiles("0.1.0", "../secret")).toThrow();
});
