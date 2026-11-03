import { useEffect, useRef, useState } from "react";
import { SourceSelect } from "@/components/source-select";
import { ListToggle, TypeToggle } from "@/components/library-toggles";
import { LibraryFilterMenu, LibraryFolders, LibrarySearch, LibrarySort } from "@/components/library-search-sort";
import { SyncPill, useSyncVisible } from "@/components/sync-pill";
import { ProgressLine } from "@/components/progress-line";
import { useLibraryFilters } from "@/lib/library-filters";
import type { LibraryCounts, ListKind, SourceGroup, SyncState } from "@/lib/library";
export function LibraryToolbar({ sources, kinds, counts, state, states, scrolled, onAddAccount, onLogin }: {
  sources: SourceGroup[]; kinds: ListKind[]; counts: LibraryCounts; state?: SyncState; states: SyncState[]; scrolled: boolean; onAddAccount: (source: "douyin" | "bilibili") => void; onLogin: (accountId: string) => void;
}) {
  const toolbar = useRef<HTMLElement>(null);
  const [layout, setLayout] = useState({ width: 0, searchCompact: true, sourceCompact: true, pillCompact: true });
  const [searchExpanded, setSearchExpanded] = useState(false);
  const visible = useSyncVisible(state);
  const narrow = layout.width < 640;
  const { scope, kind } = useLibraryFilters();
  const folderFilter = kind === "favorite" && !!scope.accountId && sources.some((source) => source.id === "bilibili" && source.accounts.some((account) => account.id === scope.accountId));
  const filtersCombined = narrow || (searchExpanded && layout.width < 880);
  useEffect(() => {
    const element = toolbar.current;
    if (!element) return;
    function measure() {
      if (!element) return;
      const css = getComputedStyle(element);
      const width = element.offsetWidth;
      const available = width - parseFloat(css.paddingLeft) - parseFloat(css.paddingRight);
      const controlWidth = (name: string) => element.querySelector<HTMLElement>(`[data-control="${name}"]`)?.offsetWidth ?? 0;
      const lists = controlWidth("lists");
      const types = controlWidth("types");
      const folders = controlWidth("folders");
      const sort = controlWidth(filtersCombined ? "filters" : "sort");
      const label = element.querySelector<HTMLElement>("[data-pill-label]")?.offsetWidth ?? 0;
      const pill = visible ? label + 38 : 0;
      const gaps = 12 + (types ? 12 : 0) + 8 + (visible ? 24 : 8);
      const sourceCompact = width < 640 || (width < 880 && 180 + lists + types + (searchExpanded ? 220 : 32) + sort + (visible ? 24 : 0) + gaps > available);
      const left = (sourceCompact ? 48 : 180) + lists + types + folders + (folders ? 12 : 0);
      const searchCompact = width < 880 || left + pill + 220 + sort + gaps > available;
      const search = searchExpanded || !searchCompact ? 220 : 32;
      const pillCompact = width < 640 || left + pill + search + sort + gaps > available;
      setLayout((old) => old.width === width && old.searchCompact === searchCompact && old.sourceCompact === sourceCompact && old.pillCompact === pillCompact
        ? old : { width, searchCompact, sourceCompact, pillCompact });
    }
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    element.querySelectorAll('[data-control="lists"], [data-control="types"], [data-pill-label], [data-control="sort"], [data-control="filters"], [data-control="folders"]').forEach((node) => observer.observe(node));
    measure();
    return () => observer.disconnect();
  }, [visible, searchExpanded, narrow, filtersCombined, folderFilter, kinds, counts, state]);
  return <header ref={toolbar} data-testid="library-toolbar" data-width={layout.width} data-search-compact={layout.searchCompact} data-search-expanded={searchExpanded} data-tier={narrow ? "narrow" : layout.width < 880 ? "medium" : "wide"} className={`library-toolbar sticky top-0 z-20 flex h-14 min-w-0 items-center border-b bg-background/85 pr-4 pl-6 backdrop-blur-md ${scrolled ? "border-border" : "border-transparent"}`}>
    <div data-control="left" className="toolbar-left flex min-w-0 shrink-0 items-center gap-3">
      <SourceSelect sources={sources} onAddAccount={onAddAccount} compact={layout.sourceCompact} /><ListToggle kinds={kinds} counts={counts} />
      {!filtersCombined && <TypeToggle />}
      {folderFilter && <LibraryFolders />}
    </div>
    <div data-control="right" className="toolbar-right ml-auto flex min-w-0 shrink-0 items-center gap-2 pl-2">
      <SyncPill state={state} states={states} visible={visible} compact={layout.pillCompact} onLogin={onLogin} />
      <div className={`min-w-0 shrink-0 ${visible ? "ml-2" : ""}`}><LibrarySearch compact={layout.searchCompact} onExpandedChange={setSearchExpanded} /></div>
      {filtersCombined ? <LibraryFilterMenu /> : <LibrarySort />}
    </div>
    <ProgressLine state={state} states={states} />
  </header>;
}
