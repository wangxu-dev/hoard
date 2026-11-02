import catalog from "./catalog.json";

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
const items = Array.from({ length: catalog.count }, (_, index) => ({
  id: `demo-work-${index + 1}`,
  kind: index % 3 === 0 ? "gallery" : "video",
  title: catalog.titles[english ? "en" : "zh-CN"][index],
  author: (english ? ["Coastal notebook", "Forest letters", "Morning walks", "Slow days"] : ["海岸手记", "山野来信", "清晨散步", "慢一点生活"])[index % 4],
  authorAvatar: `/demo-avatars/avatar-${index % 4 + 3}.webp`,
  cover: `/covers/cover-${String(index + 1).padStart(2, "0")}.webp`,
  publishedAt: 1782864000 - index * 86400 * 20,
  durationMs: index % 3 === 0 ? 0 : (45 + index * 7) * 1000,
  imageCount: 1,
  accountIds: [accounts[index % 2].id],
  likedAccountIds: [],
  favoritedAccountIds: [],
}));
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
export const Accounts = {
  list: async () => accounts,
  add: noop,
  switch: noop,
  signOut: noop,
};
export const App = {
  language: async () => (english ? "en" : "zh-CN"),
  setLanguage: noop,
  playMode: async () => "list",
  setPlayMode: noop,
  volume: async () => 1,
  setVolume: noop,
  muted: async () => false,
  setMuted: noop,
  showDouyin: noop,
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
export const Player = { media, refresh: media, warm: noop };
export const Sync = { states: async () => [], start: noop, verify: noop };
export const Actions = {
  state: async () => ({ liked: false, favorited: false }),
  set: noop,
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
