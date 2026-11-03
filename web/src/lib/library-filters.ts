import { useSyncExternalStore } from "react";
import type { ListKind, Scope, SortKey, WorkType } from "./library";

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
let state: LibraryFilters = { scope: {}, kind: "like", type: "all", query: "", queryInput: "", sort: defaultSort };
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
