import { cp, mkdir } from "node:fs/promises";
import path from "node:path";

export const releaseTargets = ["darwin-arm64", "darwin-amd64", "windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64"] as const;

export function releaseFiles(version: string, target: string): string[] {
  if (!releaseTargets.some((value) => value === target)) throw new Error(`Unsupported target: ${target}`);
  const [os, arch] = target.split("-");
  const updates = [`hoard-${version}-${target}.tar.gz`, `update-${target}.json`];
  if (os === "darwin") return [`hoard-${version}-${arch}.dmg`, ...updates];
  if (os === "windows") return [`hoard-setup-${version}-${arch}.exe`, ...updates];
  return [...updates, `hoard_${version}_${arch}.deb`];
}

if (import.meta.main) {
  const { version } = await Bun.file("mygo.json").json() as { version: string };
  const out = ".agent/release-assets";
  await mkdir(out, { recursive: true });
  if (process.argv[2] === "collect") {
    const target = process.env.RELEASE_TARGET || "";
    for (const name of releaseFiles(version, target)) {
      const source = path.join("build", target, name);
      if (!(await Bun.file(source).exists()) || Bun.file(source).size === 0) throw new Error(`Missing artifact: ${source}`);
      await cp(source, path.join(out, name));
    }
  } else if (process.argv[2] === "finalize") {
    const lines: string[] = [];
    for (const name of releaseTargets.flatMap((target) => releaseFiles(version, target)).sort()) {
      const file = Bun.file(path.join(out, name));
      if (!(await file.exists()) || file.size === 0) throw new Error(`Missing artifact: ${name}`);
      const digest = new Bun.CryptoHasher("sha256");
      for await (const chunk of file.stream()) digest.update(chunk);
      lines.push(`${digest.digest("hex")}  ${name}`);
    }
    await Bun.write(path.join(out, "SHA256SUMS"), lines.join("\n") + "\n");
    await cp("CHANGELOG.md", path.join(out, "CHANGELOG.md"));
  } else {
    throw new Error("Expected collect or finalize");
  }
}
