import type { SourceDescriptor } from "./api";
import type { ActionKind } from "./action-preview";
import type { ListKind, Scope, SourceGroup } from "./library";

export const DEFAULT_SOURCE_DESCRIPTORS: SourceDescriptor[] = [
  {
    id: "douyin",
    lists: [
      { kind: "like", role: "like", hasFolders: false },
      { kind: "favorite", role: "favorite", hasFolders: false },
      { kind: "music", role: "other", hasFolders: false },
    ],
    actions: ["like", "favorite"],
    videoActions: ["like", "favorite"],
    galleryActions: ["like", "favorite"],
    likeIcon: "heart",
    folderPicker: false,
  },
  {
    id: "bilibili",
    lists: [
      { kind: "favorite", role: "favorite", hasFolders: true },
      { kind: "watch_later", role: "queue", hasFolders: false },
    ],
    actions: ["favorite", "watch_later"],
    videoActions: ["favorite", "watch_later"],
    galleryActions: ["favorite"],
    likeIcon: "thumbs_up",
    folderPicker: true,
  },
];

const ROLE_ORDER: Record<string, number> = {
  like: 0,
  favorite: 1,
  queue: 2,
  other: 3,
};

let descriptors: SourceDescriptor[] = DEFAULT_SOURCE_DESCRIPTORS;

export function setSourceDescriptors(next: SourceDescriptor[]) {
  if (next.length > 0) descriptors = next;
}

export function getSourceDescriptors(): SourceDescriptor[] {
  return descriptors;
}

export function getSourceDescriptor(sourceId?: string): SourceDescriptor | undefined {
  const id = sourceId || "douyin";
  return descriptors.find((item) => item.id === id);
}

export function sourceHasFolders(sourceId?: string, kind: string = "favorite"): boolean {
  const descriptor = getSourceDescriptor(sourceId);
  return descriptor?.lists.some((item) => item.kind === kind && item.hasFolders) ?? false;
}

export function sourceUsesFolderPicker(sourceId: string | undefined, kind: ActionKind, workId: string): boolean {
  if (kind !== "favorite") return false;
  const descriptor = getSourceDescriptor(sourceId);
  if (!descriptor?.folderPicker) return false;
  return !workId.startsWith("opus:");
}

export function sourceActionKinds(sourceId: string | undefined, entryKind: "video" | "gallery" | "music"): ActionKind[] {
  if (entryKind === "music") return ["music"];
  const descriptor = getSourceDescriptor(sourceId);
  if (!descriptor) return ["like", "favorite"];
  const raw = entryKind === "video" ? descriptor.videoActions : descriptor.galleryActions;
  return (raw.length > 0 ? raw : descriptor.actions) as ActionKind[];
}

export function sourceLikeIcon(sourceId?: string): "heart" | "thumbs_up" {
  return getSourceDescriptor(sourceId)?.likeIcon === "thumbs_up" ? "thumbs_up" : "heart";
}

/**
 * Resolves the toolbar list tabs for the active scope:
 * - Single source/account: preserves that platform's native list order.
 * - Multi-platform aggregate ("All accounts"): orders lists by cross-platform role
 *   (like -> favorite -> queue -> other) while deduplicating shared kinds.
 */
export function visibleListKinds(sources: SourceGroup[], scope: Scope, registry: SourceDescriptor[] = descriptors): ListKind[] {
  const scoped = sources.filter((group) => !scope.source || group.id === scope.source);
  if (scoped.length === 0) return [];
  if (scoped.length === 1) return [...scoped[0].kinds];

  const roleOf = (sourceId: string, kind: string) =>
    registry.find((d) => d.id === sourceId)?.lists.find((l) => l.kind === kind)?.role ?? "other";

  const seen = new Map<ListKind, { roleRank: number; order: number }>();
  let order = 0;
  for (const group of scoped) {
    for (const kind of group.kinds) {
      const rank = ROLE_ORDER[roleOf(group.id, kind)] ?? ROLE_ORDER.other;
      const existing = seen.get(kind);
      if (!existing) seen.set(kind, { roleRank: rank, order: order++ });
      else if (rank < existing.roleRank) existing.roleRank = rank;
    }
  }
  return [...seen.entries()]
    .sort((a, b) => a[1].roleRank - b[1].roleRank || a[1].order - b[1].order)
    .map(([kind]) => kind);
}
