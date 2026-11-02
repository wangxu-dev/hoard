import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { LibraryEntry, MediaResult } from "@/lib/library";
import { mediaQuery, refreshMedia } from "@/lib/query";

export interface MediaLoad { media: MediaResult | null; error: string | null; loading: boolean; refresh: (reason?: string) => void; retry: () => void }

const placeholder = { kind: "video", id: "" } as const;

/** useMedia reads the playable media of one item from the shared cache and asks once for fresh URLs when playback fails. */
export function useMedia(entry: LibraryEntry | undefined): MediaLoad {
  const key = entry ? `${entry.kind}:${entry.id}` : "";
  const query = useQuery({ ...mediaQuery(entry ?? placeholder), enabled: !!entry });
  const [failure, setFailure] = useState<{ key: string; reason: string } | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const refreshed = useRef("");
  const media = entry ? query.data ?? null : null;
  const run = useCallback(() => {
    if (!entry) return;
    setRefreshing(true);
    refreshMedia(entry)
      .then(() => setFailure(null))
      .catch((error: unknown) => setFailure({ key, reason: error instanceof Error ? error.message : String(error) }))
      .finally(() => setRefreshing(false));
  }, [key]);
  // A gallery shown from stored images fetches its live photos and sound quietly.
  const partial = !!media && media.kind === "gallery" && !media.complete;
  useEffect(() => {
    if (!partial || !entry) return;
    refreshMedia(entry).catch(() => undefined);
  }, [partial, key]);
  const refresh = useCallback((reason = "UNAVAILABLE") => {
    if (refreshed.current === key) { setFailure({ key, reason }); return; }
    refreshed.current = key;
    run();
  }, [key, run]);
  const retry = useCallback(() => { refreshed.current = key; setFailure(null); run(); }, [key, run]);
  const queryError = query.error ? (query.error instanceof Error ? query.error.message : String(query.error)) : null;
  const error = failure?.key === key ? failure.reason : media ? null : queryError;
  return { media, error, loading: !!entry && (query.isPending || refreshing) && !error, refresh, retry };
}
