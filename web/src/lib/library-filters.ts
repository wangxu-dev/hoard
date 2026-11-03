import { useSyncExternalStore } from "react";
import type { ListKind, Scope, SortKey, SourceGroup, WorkType } from "./library";

interface LibraryFilters {
  scope: Scope;
  kind: ListKind;
  type: WorkType;
  query: string;
  queryInput: string;
  sort: SortKey;
  syncAccountId?: string;
}
export const sortOptions = [
  { value: "published", label: "toolbar.sort.published" },
  { value: "recent", label: "toolbar.sort.recent" },
] as const;
export const defaultSort: SortKey = sortOptions[0].value;
let preferredSort: SortKey = defaultSort;
let savedScope: Scope = {};
try {
  const saved: unknown = JSON.parse(localStorage.getItem("hoard-library-scope") ?? "{}");
  if (saved && typeof saved === "object") {
    const value = saved as Record<string, unknown>;
    savedScope = { source: typeof value.source === "string" ? value.source : undefined, accountId: typeof value.accountId === "string" ? value.accountId : undefined };
  }
} catch { /* Storage can be unavailable. */ }
let state: LibraryFilters = { scope: savedScope, kind: "like", type: "all", query: "", queryInput: "", sort: defaultSort };
export const libraryScope = () => state.scope;
export function reconcileLibraryScope(sources: SourceGroup[]) {
  const accounts = sources.flatMap((group) => group.accounts);
  if (!accounts.length) return;
  const selected = accounts.length === 1 ? accounts[0] : accounts.find((account) => account.id === state.scope.accountId);
  const scope = selected ? { source: selected.source, accountId: selected.id }
    : state.scope.accountId || (state.scope.source && !sources.some((group) => group.id === state.scope.source)) ? {} : state.scope;
  if (scope.source !== state.scope.source || scope.accountId !== state.scope.accountId) setLibraryFilters({ scope });
}
const listeners = new Set<() => void>();
let queryTimer: ReturnType<typeof setTimeout>;
const emit = () => listeners.forEach((fn) => fn());
export function setLibraryFilters(patch: Partial<LibraryFilters>) {
  console.debug("[hoard:ui] filters", { fields: Object.keys(patch), kind: patch.kind, type: patch.type, sort: patch.sort });
  if (patch.sort && state.kind !== "music") preferredSort = patch.sort;
  const previousScope = state.scope;
  state = { ...state, ...patch };
  if (state.kind !== "favorite" || (patch.scope && (patch.scope.accountId !== previousScope.accountId || patch.scope.source !== previousScope.source))) {
    state.scope = { ...state.scope, folderId: undefined };
  }
  state.sort = state.kind === "music" ? "recent" : preferredSort;
  if (patch.scope?.accountId) state.syncAccountId = patch.scope.accountId;
  if (patch.queryInput !== undefined) {
    clearTimeout(queryTimer);
    queryTimer = setTimeout(() => {
      state = { ...state, query: state.queryInput };
      emit();
    }, 150);
  }
  if (patch.scope) {
    try { localStorage.setItem("hoard-library-scope", JSON.stringify({ source: state.scope.source, accountId: state.scope.accountId })); } catch { /* Storage can be unavailable. */ }
  }
  emit();
}
export function useLibraryFilters() {
  return useSyncExternalStore((fn) => { listeners.add(fn); return () => { listeners.delete(fn); }; }, () => state);
}
export function useLibraryOrdering() {
  const { kind, sort } = useLibraryFilters();
  const options = sortOptions.map((option) => ({
    ...option,
    label: option.label,
    disabled: kind === "music" && option.value === "published",
    reason: "toolbar.sort.unavailable_music",
  }));
  return { sort, options, label: options.find((option) => option.value === sort)!.label };
}
