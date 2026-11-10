import { createExplorationSnapshot } from "./exploration";

const params = new URLSearchParams(globalThis.location?.search ?? "");
const english = params.get("lang") === "en";
const noop = async () => {};
const accounts = ["douyin", "bilibili"].map((source, i) => ({
  id: `demo-${source}`,
  source,
  nickname: (english ? ["Jun · On the road", "Lin · Film notes"] : ["阿峻的旅行簿", "小林的放映室"])[i],
  avatar: `/demo-avatars/avatar-${i + 1}.webp`,
  status: "active",
  active: i === 0,
}));
const seedParam = params.get("seed");
const explorationSeed = seedParam && /^\d{1,10}$/.test(seedParam) && Number(seedParam) <= 0xffffffff
  ? Number(seedParam) : globalThis.crypto.getRandomValues(new Uint32Array(1))[0];
const explorationSnapshot = createExplorationSnapshot(explorationSeed, english);
const items = explorationSnapshot.scopes.flatMap(scope => scope.works.map(work => ({
  source: work.source,
  id: work.id,
  kind: work.mediaKind === "image" ? "gallery" : work.mediaKind,
  title: work.title,
  author: work.author.name,
  authorAvatar: work.author.avatarURL,
  cover: work.cover,
  publishedAt: work.publishedAt,
  durationMs: 0,
  imageCount: 1,
  accountIds: [`demo-${work.source}`],
  likedAccountIds: [],
  favoritedAccountIds: [],
}))).sort((a, b) => b.publishedAt - a.publishedAt);
export const demoEntries = items;
type Query = {
  query?: string;
  type?: string;
  sort?: string;
  accountId?: string;
  source?: string;
  offset?: number;
  limit?: number;
  kind?: string;
};
function filtered(q: Query) {
  const query = (q.query ?? "").trim().toLocaleLowerCase();
  const result = items.filter(
    (item) =>
      (!query ||
        `${item.title} ${item.author}`.toLocaleLowerCase().includes(query)) &&
      (!q.type || q.type === "all" || q.type === item.kind) &&
      (!q.accountId || item.accountIds.includes(q.accountId)) &&
      (!q.source || item.accountIds.includes(`demo-${q.source}`)),
  );
  return q.kind === "music"
    ? []
    : q.sort === "recent"
      ? [...result].reverse()
      : result;
}
export const Library = {
  folders: async (_accountId: string) => [],
  async list(q: Query) {
    const all = filtered(q);
    return {
      items: all.slice(q.offset ?? 0, (q.offset ?? 0) + (q.limit ?? 50)),
      total: all.length,
    };
  },
  async counts(accountId = "", source = "") {
    const total = filtered({ accountId, source }).length;
    return { like: total, favorite: total, watch_later: total, music: 0 };
  },
  async timeline(q: Query) {
    const months: {
      year: number;
      month: number;
      start: number;
      count: number;
    }[] = [];
    filtered(q).forEach((item, start) => {
      const date = new Date(item.publishedAt * 1000),
        year = date.getFullYear(),
        month = date.getMonth() + 1;
      const last = months.at(-1);
      if (last?.year === year && last.month === month) last.count++;
      else months.push({ year, month, start, count: 1 });
    });
    return months;
  },
};
export const Exploration = {
  start: async () => ({ id: "website-exploration", state: "complete" }),
  status: async () => ({ id: "website-exploration", state: "complete", cacheHit: true }),
  result: async () => explorationSnapshot,
  cancel: noop,
};
export const Accounts = {
  list: async () => accounts,
  add: noop,
  switch: noop,
  signOut: noop,
};
export const App = {
  checkLogin: async () => ({ loggedIn: true }),
  language: async () => (english ? "en" : "zh-CN"),
  setLanguage: noop,
  playMode: async () => "list",
  setPlayMode: noop,
  volume: async () => 1,
  setVolume: noop,
  muted: async () => false,
  setMuted: noop,
  showDouyin: noop,
  reportExplorationDiagnostic: noop,
};
const media = async (_kind: string, id: string) => {
  const item = items.find((item) => item.id === id);
  return {
    kind: "gallery",
    videos: [],
    images: item ? [{ url: item.cover }] : [],
    music: { title: "", author: "", url: "" },
    author: item?.author ?? "",
    desc: item?.title ?? "",
    publishedAt: item?.publishedAt ?? 0,
  };
};
export const Player = { resolve: (kind: string, id: string, _accountId: string, _refresh: boolean, _partId: string) => media(kind, id), media, refresh: media, part: async (id: string, _partId: string, _refresh: boolean) => media("video", id), warm: noop };
export const Sync = { states: async () => [], autoStart: noop, start: noop, verify: noop };
export const Actions = {
  state: async () => ({ liked: false, favorited: false, watchLater: false, folders: [] }),
  folders: async (_accountId: string, _workId: string) => [],
  setFolders: async (_accountId: string, _workId: string, _ids: string[]) => ({ liked: false, favorited: false, watchLater: false, folders: [] }),
  set: async (_accountId: string, _workId: string, kind: string, on: boolean) => ({ liked: kind === "like" && on, favorited: kind === "favorite" && on, watchLater: kind === "watch_later" && on, folders: [] }),
};
export const Updates = {
  check: async () => ({
    status: "current",
    current: "0.1.5",
    nextCheckAt: Date.now() + 86400000,
  }),
  downloadStatus: async () => ({ status: "idle", enabled: false }),
  download: noop,
  restart: noop,
  open: noop,
};
const event = { on: () => () => {} };
export const events = {
  accountsChanged: event,
  libraryChanged: event,
  syncProgress: event,
};
