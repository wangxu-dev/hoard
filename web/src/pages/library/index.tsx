import { useCallback, useEffect, useRef, useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { LibraryToolbar } from "@/components/library-toolbar";
import { getLibrarySource, type LibraryCounts, type LibraryEntry, type ListKind, type SourceGroup, type SyncState, type TimelineMonth } from "@/lib/library";
import { LibraryGrid, type GridGeometry } from "@/components/library-grid";
import { TimelineScrubber } from "@/components/timeline-scrubber";
import { Viewer } from "@/components/viewer/viewer";
import { ListCursor } from "@/lib/library-cursor";
import { setLibraryFilters, useLibraryFilters, useLibraryOrdering } from "@/lib/library-filters";
import { useLibraryVersion } from "@/lib/library-events";
import { queryKeys } from "@/lib/query";
import { playMusic } from "@/lib/music";

const noSources: SourceGroup[] = [];
const noStates: SyncState[] = [];
const noCounts: LibraryCounts = { like: 0, favorite: 0, music: 0, watch_later: 0 };
const noMonths: TimelineMonth[] = [];

export function LibraryPage({ scrolled, onAddAccount, onLogin }: { scrolled: boolean; onAddAccount: (source: "douyin" | "bilibili") => void; onLogin: (accountId: string) => void }) {
  const { t } = useTranslation("library");
  const filters = useLibraryFilters();
  const ordering = useLibraryOrdering();
  const source = getLibrarySource();
  // Lists, counts and sync status come from the app-wide cache, so returning to
  // this page renders at once and only refreshes in the background.
  const sourcesQuery = useQuery({ queryKey: queryKeys.sources, queryFn: () => source.sources(), staleTime: 60_000 });
  const statesQuery = useQuery({ queryKey: queryKeys.syncStates, queryFn: () => source.syncStates(), staleTime: Infinity });
  const countsQuery = useQuery({ queryKey: queryKeys.counts(filters.scope), queryFn: () => source.counts(filters.scope), placeholderData: keepPreviousData });
  const sources = sourcesQuery.data ?? noSources;
  useQuery({
    queryKey: ["account-session", filters.scope.accountId ?? ""],
    queryFn: async ({ signal }) => { await source.checkSession(filters.scope.accountId, signal); if (!signal.aborted) await sourcesQuery.refetch(); return null; },
    staleTime: 0, refetchOnMount: "always", refetchOnWindowFocus: "always",
  });
  const [dismissedLogin, setDismissedLogin] = useState<string | null>(null);
  const selectedAccount = sources.flatMap((group) => group.accounts).find((account) => filters.scope.accountId ? account.id === filters.scope.accountId : account.active);
  const signedOut = selectedAccount?.status === "signed_out" ? selectedAccount : undefined;
  useEffect(() => { setDismissedLogin(null); }, [selectedAccount?.id, selectedAccount?.status]);
  const states = statesQuery.data ?? noStates;
  const counts = { ...noCounts, ...countsQuery.data };
  const error = (sourcesQuery.isError && !sourcesQuery.data) || (countsQuery.isError && !countsQuery.data);
  const reload = () => { void sourcesQuery.refetch(); void statesQuery.refetch(); void countsQuery.refetch(); };
  const version = useLibraryVersion();
  // Months only exist for lists in publish order; music has no publish time.
  const timelineQuery = { scope: filters.scope, kind: filters.kind, type: filters.kind === "music" ? "all" as const : filters.type, query: filters.query, sort: ordering.sort };
  const dated = filters.kind !== "music" && ordering.sort === "published";
  const monthsQuery = useQuery({ queryKey: queryKeys.timeline(JSON.stringify([filters.scope.accountId, filters.scope.source, filters.scope.folderId, timelineQuery.kind, timelineQuery.type, filters.query, ordering.sort])),
    queryFn: () => source.timeline(timelineQuery), placeholderData: keepPreviousData, enabled: dated });
  const [geometry, setGeometry] = useState<GridGeometry | null>(null);
  const [viewer, setViewer] = useState<{ cursor: ListCursor; index: number; from: LibraryEntry } | null>(null);
  useEffect(() => { if (signedOut) setViewer(null); }, [signedOut?.id]);
  const [visibleVersion, setVisibleVersion] = useState(version);
  useEffect(() => { if (!viewer) setVisibleVersion(version); }, [version, viewer]);
  const items = useRef<(LibraryEntry | undefined)[]>([]);
  const total = useRef(0);
  const handleItems = useCallback((next: (LibraryEntry | undefined)[]) => { items.current = next; }, []);
  const handleTotal = useCallback((next: number) => { total.current = next; }, []);
  // The viewer and the music player walk the same filtered, sorted list the grid shows.
  const openEntry = useCallback((entry: LibraryEntry) => {
    const cursor = new ListCursor({ scope: filters.scope, kind: filters.kind, type: filters.kind === "music" ? "all" : filters.type, query: filters.query, sort: ordering.sort }, items.current, total.current);
    const index = Math.max(0, cursor.indexOf(entry));
    if (entry.kind === "music") playMusic(cursor, index);
    else setViewer({ cursor, index, from: entry });
  }, [filters.kind, filters.scope, filters.query, filters.type, ordering.sort]);
  const closeViewer = useCallback(() => {
    const from = viewer?.from;
    setViewer(null);
    // Focus returns to the card that opened the viewer without moving the grid.
    if (from) requestAnimationFrame(() => (document.querySelector(`[data-entry="${from.kind}:${CSS.escape(from.id)}"]`) as HTMLElement | null)?.focus({ preventScroll: true }));
  }, [viewer]);
  const kinds = [...new Set(sources.filter((s) => !filters.scope.source || s.id === filters.scope.source).flatMap((s) => s.kinds))];
  useEffect(() => { if (kinds.length && !kinds.includes(filters.kind)) setLibraryFilters({ kind: kinds[0] }); }, [kinds.join(","), filters.kind]);
  const syncAccount = filters.scope.accountId ?? filters.syncAccountId ?? sources.flatMap((g) => g.accounts)[0]?.id;
  const accountStates = states.filter((s) => s.accountId === syncAccount && s.kind !== "cover");
  // Attention states belong to the whole account; otherwise the pill follows the selected tab.
  const state = accountStates.find((s) => s.status === "signed_out" || s.status === "blocked" || s.status === "verifying" || s.status === "cooling")
    ?? accountStates.find((s) => s.kind === filters.kind) ?? accountStates[0];
  const syncing = accountStates.some((s) => s.status === "running" || s.status === "queued" || s.status === "cooling");
  const displayCounts = { ...counts };
  // The right gutter is kept free for the timeline; the grid centers between equal margins.
  return <section className="relative flex min-h-full min-w-0 flex-1 bg-background">
    <div data-testid="library-main" className="min-w-0 flex-1 bg-background">
      <LibraryToolbar sources={sources} kinds={kinds as ListKind[]} counts={displayCounts} state={filters.scope.accountId ? state : undefined} states={states} scrolled={scrolled} onAddAccount={onAddAccount} onLogin={onLogin} />
      <div className="min-w-0 px-12 py-4">
        {error ? <div role="alert">{t("error.load")}<Button variant="outline" size="sm" className="ml-3" onClick={reload}>{t("error.retry")}</Button></div>
          : <LibraryGrid scope={filters.scope} kind={filters.kind} type={filters.type} query={filters.query} sort={ordering.sort} version={visibleVersion} syncing={syncing} onOpen={openEntry} onItemsChange={handleItems} onTotalChange={handleTotal} onGeometry={setGeometry} />}
      </div>
    </div>
    <div data-testid="timeline-reserved" className="pointer-events-none absolute top-0 right-0 h-full w-12">
      <TimelineScrubber months={dated ? monthsQuery.data ?? noMonths : noMonths} geometry={geometry} active={dated && !error} />
    </div>
    <AlertDialog open={!!signedOut && dismissedLogin !== signedOut.id} onOpenChange={(open) => { if (!open && signedOut) setDismissedLogin(signedOut.id); }}>
      <AlertDialogContent>
        <AlertDialogHeader><AlertDialogTitle>{t("session.title")}</AlertDialogTitle><AlertDialogDescription>{t("session.description", { account: signedOut?.nickname })}</AlertDialogDescription></AlertDialogHeader>
        <AlertDialogFooter><AlertDialogCancel>{t("session.later")}</AlertDialogCancel><AlertDialogAction onClick={() => { if (signedOut) onLogin(signedOut.id); }}>{t("session.login")}</AlertDialogAction></AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    {viewer && <Viewer key={`${viewer.from.kind}:${viewer.from.id}`} cursor={viewer.cursor} start={viewer.index} onClose={closeViewer} />}
  </section>;
}
