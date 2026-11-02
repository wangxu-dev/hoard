import { mkdir, appendFile } from "node:fs/promises";
import { validateRelease } from "./release-notes";

const { version } = await Bun.file("mygo.json").json() as { version: string };
if (process.env.GITHUB_REF_TYPE !== "tag" || !process.env.GITHUB_REF_NAME) throw new Error("Release requires a version tag");
const tag = process.env.GITHUB_REF_NAME;
const notes = validateRelease(tag, version, await Bun.file("CHANGELOG.md").text());
const result = Bun.spawnSync(["git", "rev-parse", "HEAD"]);
if (result.exitCode !== 0) throw new Error("Cannot resolve release commit");
const sha = result.stdout.toString().trim();
await mkdir(".agent", { recursive: true });
await Bun.write(".agent/release-notes.md", notes + "\n");
if (process.env.GITHUB_OUTPUT) {
  await appendFile(process.env.GITHUB_OUTPUT, `tag=${tag}\nversion=${version}\nsha=${sha}\n`);
}
console.log(`Validated ${tag} at ${sha}`);
