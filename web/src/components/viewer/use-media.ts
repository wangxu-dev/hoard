import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { LibraryEntry, MediaResult } from "@/lib/library";
import { mediaQuery, refreshMedia } from "@/lib/query";

export interface MediaLoad { media: MediaResult | null; error: string | null; loading: boolean; refresh: (reason?: string) => void; retry: () => void }

const placeholder = { kind: "video", id: "" } as const;
const permanent = new Set(["DELETED", "PRIVATE", "REVIEWING", "WITHHELD", "NEED_VERIFY", "COOLING"]);

/** Resolve media and automatically request fresh URLs once after a load or playback failure. */
export function useMedia(entry: LibraryEntry | undefined): MediaLoad {
  const key = entry ? `${entry.kind}:${entry.id}` : "";
  const query = useQuery({ ...mediaQuery(entry ?? placeholder), enabled: !!entry });
  const [failure, setFailure] = useState<{ key: string; reason: string } | null>(null);
  const [refreshingKey, setRefreshingKey] = useState<string | null>(null);
  const attempt = useRef({ key, refreshed: false, pending: false });
  if (attempt.current.key !== key) attempt.current = { key, refreshed: false, pending: false };
  const media = entry ? query.data ?? null : null;
  const run = useCallback(() => {
    const current = attempt.current;
    if (!entry || current.pending) return;
    current.pending = true;
    current.refreshed = true;
    setFailure(null);
    setRefreshingKey(key);
    refreshMedia(entry)
      .then(() => { if (attempt.current === current) setFailure(null); })
      .catch((error: unknown) => {
        if (attempt.current === current) setFailure({ key, reason: error instanceof Error ? error.message : String(error) });
      })
      .finally(() => {
        current.pending = false;
        if (attempt.current === current) setRefreshingKey(null);
      });
  }, [key]);
  // Stored galleries show their images while the detail fills in sound and live photos.
  const partial = !!media && media.kind === "gallery" && !media.complete;
  useEffect(() => { if (partial) run(); }, [partial, run]);
  const refresh = useCallback((reason = "UNAVAILABLE") => {
    if (attempt.current.pending) return;
    if (attempt.current.refreshed) { setFailure({ key, reason }); return; }
    run();
  }, [key, run]);
  const queryError = query.error ? (query.error instanceof Error ? query.error.message : String(query.error)) : null;
  useEffect(() => {
    if (queryError && !media && !permanent.has(queryError) && !attempt.current.refreshed) run();
  }, [queryError, media, run]);
  const refreshing = refreshingKey === key;
  const error = failure?.key === key ? failure.reason : refreshing ? null : media ? null : queryError;
  return { media, error, loading: !!entry && (query.isPending || refreshing) && !error, refresh, retry: run };
}
