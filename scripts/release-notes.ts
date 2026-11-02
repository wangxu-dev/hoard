export function releaseNotes(markdown: string, version: string): string {
  const sections = markdown.split(/^##\s+/m).slice(1);
  const section = sections.find((value) => {
    const heading = value.split("\n", 1)[0]!.trim();
    return heading === version || heading === `[${version}]` || heading.startsWith(`[${version}] - `);
  });
  const notes = section?.includes("\n") ? section.slice(section.indexOf("\n") + 1).trim() : "";
  if (!notes) throw new Error(`CHANGELOG.md needs a non-empty section for ${version}`);
  return notes;
}

export function validateRelease(tag: string, version: string, markdown: string): string {
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(version)) {
    throw new Error("Invalid release version");
  }
  if (tag !== `v${version}`) throw new Error(`Tag must match mygo.json: v${version}`);
  return releaseNotes(markdown, version);
}
