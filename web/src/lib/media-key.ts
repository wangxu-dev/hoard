/**
 * Music tracks are resolved from the local store regardless of which account
 * scope is currently selected in the library toolbar.
 */
export function mediaScopeAccount(kind: string, accountId?: string): string {
  return kind === "music" ? "" : (accountId ?? "");
}

export interface TrackSrc {
  id: string;
  url: string;
}

/**
 * Keeps the active audio URL locked for the current track so background media
 * refetches never reset playback to 00:00. Only a track switch or explicit
 * recovery after an audio error replaces the URL.
 */
export function nextTrackSrc(
  prev: TrackSrc | null,
  entryId: string | undefined,
  incomingUrl: string,
  recovering: boolean,
): TrackSrc | null {
  if (!entryId || !incomingUrl) return prev;
  if (prev && prev.id === entryId && !recovering) return prev;
  return { id: entryId, url: incomingUrl };
}
