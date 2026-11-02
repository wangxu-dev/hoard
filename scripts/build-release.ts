import { cp, mkdir, chmod } from "node:fs/promises";

const build = Bun.spawn([
  "bun", "x", "--no-install", "mygo", "build",
  "-platform", "darwin/arm64,windows/amd64,linux/amd64,linux/arm64",
], { stdout: "inherit", stderr: "inherit" });
const code = await build.exited;
if (code !== 0) process.exit(code);

await mkdir("release", { recursive: true });
for (const [source, target] of [
  ["build/darwin-arm64/Hoard.app", "release/Hoard-macOS-arm64.app"],
  ["build/windows-amd64/Hoard.exe", "release/Hoard-Windows-x64.exe"],
  ["build/linux-amd64/hoard", "release/Hoard-Linux-x64"],
  ["build/linux-arm64/hoard", "release/Hoard-Linux-arm64"],
]) {
  await cp(source, target, { recursive: true, force: true });
  if (!target.endsWith(".app")) await chmod(target, 0o755);
}
console.log("Release files are ready in release/");
