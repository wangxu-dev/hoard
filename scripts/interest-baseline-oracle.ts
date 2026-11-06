// Regression oracle only: never imported by client runtime.
import definitions from "../internal/explore/rules.json";
import type { LibraryItem } from "@mygo";

export type Source = "douyin" | "bilibili";
export type Rule = { id: string; label: string; sources: string[]; clues: string[]; reason: string };
export const ANALYSIS_VERSION = "literal-baseline-v1";
export const RULES: Rule[] = definitions;
export const clean = (text: string) => text.replace(/https?:\/\/\S+|www\.\S+/gi, " ").replace(/@[^\s#，。！？,!?：:]+/g, " ").replace(/\s+/g, " ").trim().toLowerCase();
export function clueMatch(text: string, clue: string): boolean {
  const c = clue.toLowerCase();
  if (/^[a-z0-9_+#.\-]+$/.test(c)) {
    const safe = c.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return new RegExp(`(^|[^a-z0-9])${safe}($|[^a-z0-9])`).test(text);
  }
  return c.length >= 2 && text.includes(c);
}
export function cachedAsset(ref: string): string {
  if (!ref.startsWith("hoard-cover://")) return "";
  return ref + (ref.includes("?") ? "&" : "?") + "cacheOnly=1";
}
export type Membership = { workId: string; topicId: string; method: string; confidence: string; reason: string; evidence: { field: string; excerpt: string }[] };
export function buildScope(items: LibraryItem[], source: Source) {
  const works = items.map((w) => ({
    id: w.id, source, title: w.title || "", mediaKind: w.kind === "gallery" ? "image" : "video",
    textAvailability: w.title.trim() ? "ok" : "none",
    cover: cachedAsset(w.cover), author: { ...(w.authorId ? { id: `${source}:${w.authorId}` } : {}), name: w.author || "", avatarURL: cachedAsset(w.authorAvatar) },
    // Author identity is source-scoped and native; missing identity is never inferred from nickname.
    // No fabricated source_saved_at or observed historical month.
    monthKeys: { source_time: null, observed_time: null }, times: [],
  }));
  const membership: Membership[] = [];
  const topics: { id: string; label: string; kind: string; workCount: number; workIds: string[] }[] = [];
  for (const rule of RULES.filter((r) => r.sources.includes(source))) {
    const matches = works.map((w) => ({ w, hits: rule.clues.filter((c) => clueMatch(clean(w.title), c)) })).filter((x) => x.hits.length);
    if (matches.length < 3) continue;
    topics.push({ id: rule.id, label: rule.label, kind: "topic", workCount: matches.length, workIds: matches.map(({ w }) => w.id) });
    for (const { w, hits } of matches) membership.push({ workId: w.id, topicId: rule.id, method: "literal_contains", confidence: "medium", reason: "标题含：" + hits.slice(0, 4).join("、"), evidence: hits.slice(0, 4).map((h) => ({ field: "title", excerpt: h })) });
  }
  const assigned = new Set(membership.map((m) => m.workId));
  const noText = works.filter((w) => w.textAvailability === "none"), unknown = works.filter((w) => w.textAvailability === "ok" && !assigned.has(w.id));
  for (const [id, label, list, reason] of [["t_uncategorized", "未归类", unknown, "未命中主题规则"], ["t_no_text", "无文案", noText, "无文案，无法规则归类"]] as const) {
    topics.push({ id, label, kind: "bucket", workCount: list.length, workIds: list.map((w) => w.id) });
    for (const w of list) membership.push({ workId: w.id, topicId: id, method: "fallback", confidence: "low", reason, evidence: [] });
  }
  const edges: { source: string; target: string; coCount: number; jaccard: number; method: string }[] = [];
  const real = topics.filter((t) => t.kind === "topic");
  for (let i = 0; i < real.length; i++) for (let j = i + 1; j < real.length; j++) {
    const a = real[i], b = real[j], bs = new Set(b.workIds), co = a.workIds.filter((id) => bs.has(id)).length, jaccard = co / (a.workCount + b.workCount - co);
    if (co >= 2 && jaccard >= .03) edges.push({ source: a.id, target: b.id, coCount: co, jaccard, method: "co_membership_jaccard" });
  }
  const coverage = { worksTotal: works.length, worksInTopics: assigned.size, uncategorizedWorks: unknown.length, worksNoText: noText.length, authorNames: works.filter((w) => w.author.name).length, authorIds: works.filter((w) => !!w.author.id).length, avatarRefs: works.filter((w) => w.author.avatarURL).length, coverRefs: works.filter((w) => w.cover).length };
  return { source, works, topics, topicMembership: membership, edges: edges.sort((a, b) => b.jaccard - a.jaccard).slice(0, 40), coverage, timeCapabilities: { views: { current: true, source_time: false, observed_time: false } } };
}
export async function digest(value: unknown): Promise<string> {
  const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(JSON.stringify(value)));
  return Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, "0")).join("");
}
export async function analyze(items: LibraryItem[], signal?: AbortSignal) {
  if (signal?.aborted) throw new Error("CANCELLED");
  const sorted = [...items].sort((a, b) => `${a.source}:${a.id}`.localeCompare(`${b.source}:${b.id}`));
  const ruleHash = await digest([ANALYSIS_VERSION, RULES]);
  const inputRevision = await digest(sorted.map((w) => [w.source, w.id, w.title, w.author, w.authorId, w.authorAvatar, w.cover, w.kind]));
  if (signal?.aborted) throw new Error("CANCELLED");
  const scopes = (["douyin", "bilibili"] as Source[]).map((source) => buildScope(sorted.filter((w) => w.source === source), source));
  return { schemaVersion: 1, analysisVersion: ANALYSIS_VERSION, snapshotId: `current-${inputRevision.slice(0, 20)}-${ruleHash.slice(0, 8)}`, inputRevision, ruleHash, snapshotKind: "baseline", createdAt: new Date().toISOString(), meta: { mode: "real", input: "native Library.List", coverage: scopes.map((s) => ({ source: s.source, ...s.coverage })), persistence: "memory-only", consistency: "sequential read guarded by libraryChanged and total checks; not a database transaction" }, scopes };
}
