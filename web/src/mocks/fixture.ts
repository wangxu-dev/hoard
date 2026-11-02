import type { LibrarySource, SourceGroup, SyncState } from "@/lib/library";
import { fixtureFrozen, type FixtureName } from "@/lib/demo";

function mulberry32(seed: number) {
  return () => {
    let value = seed += 0x6D2B79F5;
    value = Math.imul(value ^ value >>> 15, value | 1);
    value ^= value + Math.imul(value ^ value >>> 7, value | 61);
    return ((value ^ value >>> 14) >>> 0) / 4294967296;
  };
}
export function createFixtureSource(name: FixtureName): LibrarySource {
const random = mulberry32(20261031);
function avatar(letter: string) {
  const color = ["slateblue", "seagreen", "steelblue", "indianred"][Math.floor(random() * 4)];
  return `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><defs><linearGradient id="a"><stop stop-color="${color}"/><stop offset="1" stop-color="darkslateblue"/></linearGradient></defs><rect width="40" height="40" rx="20" fill="url(#a)"/><text x="20" y="25" text-anchor="middle" fill="white" font-family="sans-serif" font-size="16">${letter}</text></svg>`)}`;
}
const sources: SourceGroup[] = [{ id: "douyin", name: "抖音", kinds: ["like", "favorite", "music"], accounts: [
  { id: "demo-a", source: "douyin", nickname: "示例账号 A", avatarUrl: avatar("A"), status: "active", syncing: true },
  { id: "demo-b", source: "douyin", nickname: "示例账号 B", avatarUrl: avatar("B"), status: "signed_out", syncing: false },
] }];
  let state: SyncState = { accountId: name === "login-required" ? "demo-b" : "demo-a", kind: "like", phase: name === "synced" ? "done" : "idle-real-layout" === name ? "idle" : "backfill",
    status: name === "login-required" ? "signed_out" : name === "idle-real-layout" ? "idle" : name === "cooling" ? "cooling" : name === "synced" ? "idle" : "running",
    items: name === "unknown-total" ? 340 : name === "synced" ? 10000 : 1240,
    remoteTotal: name === "unknown-total" ? undefined : 10000, pages: 62,
    etaSeconds: name === "unknown-total" ? undefined : 1080,
    retryAt: name === "cooling" ? Math.floor(Date.now() / 1000) + 118 : undefined };
  return {
    async sources() { return sources.map((g) => ({ ...g, accounts: g.accounts.map((a) => ({ ...a, syncing: a.id === "demo-a" && state.status === "running" })) })); },
    async counts(scope) { return scope.accountId === "demo-a" ? { like: 10000, favorite: 56, music: 78 }
      : scope.accountId === "demo-b" ? { like: 1234, favorite: 12, music: 0 } : { like: 10860, favorite: 68, music: 78 }; },
    async syncStates() { return [state,
      { accountId: "demo-a", kind: "cover", phase: "head", status: "running", items: 210, pages: 0 },
      { accountId: "demo-a", kind: "favorite", phase: "idle", status: "idle", items: 0, pages: 0 }]; },
    onSyncProgress(fn) {
      if (fixtureFrozen || name !== "indexing") return () => {};
      const timer = setInterval(() => {
        state = { ...state, items: Math.min(10000, state.items + 20), pages: state.pages + 1 };
        if (state.items === 10000) state = { ...state, phase: "done", status: "idle" };
        fn(state);
      }, 2600);
      return () => clearInterval(timer);
    },
    async list(q) {
      const total = q.kind === "music" ? 78 : 240;
      const items = Array.from({ length: Math.max(0, Math.min(q.limit, total - q.offset)) }, (_, i) => {
        const n = q.offset + i;
        return { id: `demo-${q.kind}-${n}`, kind: q.kind === "music" ? "music" as const : n % 4 === 0 ? "gallery" as const : "video" as const,
          title: `示例作品 ${n + 1}`, author: `作者 ${(n % 9) + 1}`, cover: avatar(String((n % 9) + 1)), publishedAt: 1767000000 - n * 3600, durationMs: 15000 + n * 1000, imageCount: n % 4 === 0 ? 3 : 0 };
      });
      return { items, total };
    },
    async timeline(q) {
      if (q.kind === "music" || q.sort !== "published") return [];
      // Hourly items, newest first, grouped by local month.
      const months: { year: number; month: number; count: number; start: number }[] = [];
      for (let n = 0; n < 240; n++) {
        const d = new Date((1767000000 - n * 3600 * 24) * 1000);
        const last = months[months.length - 1];
        if (last && last.year === d.getFullYear() && last.month === d.getMonth() + 1) last.count++;
        else months.push({ year: d.getFullYear(), month: d.getMonth() + 1, count: 1, start: n });
      }
      return months;
    },
    onLibraryChanged() { return () => {}; },
    async media(kind, id) { return { kind: kind as "video", videos: kind === "music" ? [] : [avatar(id)], images: kind === "gallery" ? [{ url: avatar(id) }] : [], music: { title: "示例配乐", author: "示例作者", url: "" }, author: "示例作者", desc: "示例文案", publishedAt: 1767000000 }; },
    warm() { return Promise.resolve(); },
    playMode() { return Promise.resolve("list" as const); },
    setPlayMode() { return Promise.resolve(); },
    volume() { return Promise.resolve(1); },
    setVolume() { return Promise.resolve(); },
    muted() { return Promise.resolve(false); },
    setMuted() { return Promise.resolve(); },
    async startSync() {},
    async verify() {},
  };
}
