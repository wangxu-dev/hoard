import { queryOptions } from "@tanstack/react-query";
import { libraryScope } from "./library-filters";
import { mediaScopeAccount } from "./media-key";
import { getLibrarySource, type LibraryEntry, type MediaResult, type Scope } from "./library";
import { queryClient } from "./query-client";

export { queryClient };

export const queryKeys = {
  sources: ["sources"] as const,
  syncStates: ["sync-states"] as const,
  counts: (scope: Scope) => ["counts", scope.source ?? "", scope.accountId ?? ""] as const,
  grid: (filterKey: string) => ["grid", filterKey] as const,
  timeline: (filterKey: string) => ["timeline", filterKey] as const,
  media: (kind: string, id: string, partId?: string, accountId = libraryScope().accountId ?? "") => ["media", accountId, kind, id, partId ?? ""] as const,
};

/** Folder names come from SQLite; sync and successful actions invalidate this cache. */
export function foldersQuery(accountId: string) {
  return queryOptions({
    queryKey: ["folders", accountId],
    queryFn: () => getLibrarySource().folders(accountId),
    staleTime: Infinity,
  });
}

// Play URLs are reused for half an hour, or until the CDN says they expire.
const MEDIA_FRESH = 30 * 60_000;
function mediaFreshness(media: MediaResult | undefined) {
  if (!media) return 0;
  // A gallery shown from stored images still needs its sound and live photos.
  if (media.kind === "gallery" && !media.complete) return 0;
  return media.expiresAt ? Math.max(0, Math.min(MEDIA_FRESH, media.expiresAt - Date.now())) : MEDIA_FRESH;
}

/** mediaQuery resolves the playable media of one item. */
export function mediaQuery(entry: Pick<LibraryEntry, "kind" | "id">, full = false, partId?: string) {
  const accountId = mediaScopeAccount(entry.kind, libraryScope().accountId);
  return queryOptions({
    queryKey: queryKeys.media(entry.kind, entry.id, partId, accountId),
    queryFn: () => getLibrarySource().media(entry.kind, entry.id, full, partId, accountId),
    staleTime: (query) => mediaFreshness(query.state.data),
    gcTime: MEDIA_FRESH,
    structuralSharing: false,
  });
}

/** refreshMedia asks for new play URLs and replaces the cached ones. */
export async function refreshMedia(entry: Pick<LibraryEntry, "kind" | "id">, partId?: string): Promise<MediaResult> {
  const accountId = mediaScopeAccount(entry.kind, libraryScope().accountId);
  const media = await getLibrarySource().media(entry.kind, entry.id, true, partId, accountId);
  queryClient.setQueryData(queryKeys.media(entry.kind, entry.id, partId, accountId), media);
  return media;
}

/** prefetchMedia resolves an item the viewer will likely show next, including a gallery's sound. */
export function prefetchMedia(entry: Pick<LibraryEntry, "kind" | "id">) {
  return queryClient.fetchQuery(mediaQuery(entry, true));
}
