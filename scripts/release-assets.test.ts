import { expect, test } from "bun:test";
import { releaseFiles, releaseTargets } from "./release-assets";

test("six architectures produce distinct release assets", () => {
  const names = releaseTargets.flatMap((target) => releaseFiles("0.1.0", target));
  expect(names).toHaveLength(8);
  expect(new Set(names).size).toBe(8);
  expect(releaseFiles("0.1.0", "windows-arm64")).toEqual(["hoard-setup-0.1.0-arm64.exe"]);
  expect(releaseFiles("0.1.0", "darwin-amd64")).toEqual(["hoard-0.1.0-amd64.dmg"]);
  expect(() => releaseFiles("0.1.0", "../secret")).toThrow();
});
