import { Accounts, App, Library, Player, Sync, onLibraryChanged, onSyncProgress } from "./api";
import type { SyncView } from "./api";
import i18n from "@/i18n";
import { fixtureName } from "./demo";
import { createFixtureSource } from "@/mocks/fixture";

export type ListKind = "like" | "favorite" | "music";
export type WorkType = "all" | "video" | "gallery";
export type SortKey = "published" | "recent";
export type PlayMode = "list" | "one" | "shuffle";
export interface Scope { source?: string; accountId?: string }
export interface SourceAccount {
  id: string; source: string; nickname: string; avatarUrl: string;
  status: "active" | "signed_out" | "pending"; syncing: boolean;
}
export interface SourceGroup { id: string; name: string; kinds: ListKind[]; accounts: SourceAccount[] }
export interface SyncState {
  accountId: string; kind: ListKind | "cover";
  phase: "idle" | "count" | "head" | "backfill" | "done";
  status: "running" | "checking" | "queued" | "paused" | "cooling" | "blocked" | "verifying" | "signed_out" | "unsupported" | "idle";
  items: number; remoteTotal?: number; pages: number; retryAt?: number; etaSeconds?: number; errorCode?: string; lastSyncAt?: number; fresh?: number;
}
export interface LibraryCounts { like: number; favorite: number; music: number }
export interface LibraryEntry {
  id: string; kind: "video" | "gallery" | "music"; title: string; author: string; cover: string; authorAvatar?: string;
  publishedAt: number; durationMs: number; imageCount: number; accountIds?: string[];
}
export interface MediaImage { url: string; liveUrl?: string }
export interface MediaResult { kind: "video" | "gallery" | "music"; videos: string[]; images: MediaImage[]; music: { title: string; author: string; url: string }; author: string; authorAvatar?: string; desc: string; publishedAt: number; complete?: boolean; expiresAt?: number }
/** One month of a list sorted by publish time; start is the offset of its first item. Year and month are 0 for items without a date. */
export interface TimelineMonth { year: number; month: number; count: number; start: number }
export interface ListQuery { scope: Scope; kind: ListKind; type: WorkType; query: string; sort: SortKey; offset: number; limit: number }
export interface LibrarySource {
  sources(): Promise<SourceGroup[]>;
  counts(scope: Scope): Promise<LibraryCounts>;
  syncStates(): Promise<SyncState[]>;
  onSyncProgress(fn: (s: SyncState) => void): () => void;
  list(query: ListQuery): Promise<{ items: LibraryEntry[]; total: number }>;
  timeline(query: Omit<ListQuery, "offset" | "limit">): Promise<TimelineMonth[]>;
  media(kind: string, id: string, refresh?: boolean): Promise<MediaResult>;
  warm(urls: string[]): Promise<void>;
  playMode(): Promise<PlayMode>;
  setPlayMode(value: PlayMode): Promise<void>;
  volume(): Promise<number>;
  setVolume(value: number): Promise<void>;
  muted(): Promise<boolean>;
  setMuted(value: boolean): Promise<void>;
  onLibraryChanged(fn: (accountId: string, kind: ListKind) => void): () => void;
  startSync(accountId: string): Promise<void>;
  verify(): Promise<void>;
}
function toState(view: SyncView): SyncState {
  return {
    accountId: view.accountId, kind: view.kind as ListKind,
    phase: (["idle", "head", "backfill", "done"].includes(view.phase) ? view.phase : "idle") as SyncState["phase"],
    status: view.status as SyncState["status"], items: view.items, pages: view.pages,
    remoteTotal: view.remoteTotal >= 0 ? view.remoteTotal : undefined,
    retryAt: view.retryAt > 0 ? view.retryAt : undefined,
    errorCode: view.errorCode || undefined,
    lastSyncAt: view.lastSyncAt > 0 ? view.lastSyncAt : undefined,
    fresh: view.fresh > 0 ? view.fresh : undefined,
  };
}
const realSource: LibrarySource = {
  async sources() {
    const accounts = await Accounts.list();
    return [{ id: "douyin", name: i18n.t("library:toolbar.source.douyin"), kinds: ["like", "favorite", "music"], accounts: accounts.filter((a) => a.status !== "pending").map((a) => ({
      id: a.id, source: "douyin", nickname: a.nickname, avatarUrl: a.avatar,
      status: a.status === "signed_out" ? "signed_out" : a.status === "pending" ? "pending" : "active", syncing: false,
    })) }];
  },
  async counts(scope) {
    return Library.counts(scope.accountId ?? "", scope.source ?? "");
  },
  async syncStates() {
    return (await Sync.states()).map(toState);
  },
  onSyncProgress(fn) { return onSyncProgress((view) => fn(toState(view))); },
  async list(q) {
    const page = await Library.list({ accountId: q.scope.accountId ?? "", source: q.scope.source ?? "", kind: q.kind, type: q.type, query: q.query, sort: q.sort, offset: q.offset, limit: q.limit });
    return { items: page.items as LibraryEntry[], total: page.total };
  },
  async timeline(q) {
    return (await Library.timeline({ accountId: q.scope.accountId ?? "", source: q.scope.source ?? "", kind: q.kind, type: q.type, query: q.query, sort: q.sort, offset: 0, limit: 0 })) ?? [];
  },
  onLibraryChanged(fn) { return onLibraryChanged((change) => fn(change.accountId, change.kind as ListKind)); },
  warm(urls) { return Player.warm(urls); },
  async playMode() { const mode = await App.playMode(); return mode === "one" || mode === "shuffle" ? mode : "list"; },
  setPlayMode(value) { return App.setPlayMode(value); },
  media(kind, id, refresh = false) { return (refresh ? Player.refresh(kind, id) : Player.media(kind, id)) as Promise<MediaResult>; },
  volume() { return App.volume(); },
  setVolume(value) { return App.setVolume(value); },
  muted() { return App.muted(); },
  setMuted(value) { return App.setMuted(value); },
  async startSync(accountId) {
    const accounts = await Accounts.list();
    if (!accounts.some((account) => account.id === accountId && account.active)) await Accounts.switch(accountId);
    await Sync.start();
  },
  verify() { return Sync.verify(); },
};
const source = import.meta.env.DEV && fixtureName ? createFixtureSource(fixtureName) : realSource;
export function getLibrarySource(): LibrarySource { return source; }
