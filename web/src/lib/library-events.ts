import { useSyncExternalStore } from "react";
import { onAccountsChanged } from "./api";
import { getLibrarySource, type SyncState } from "./library";
import { queryClient, queryKeys } from "./query";

// Library changes are counted app-wide, so a page that mounts later still sees them.
let version = 0;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((fn) => fn());

export function useLibraryVersion(): number {
  return useSyncExternalStore((fn) => { listeners.add(fn); return () => { listeners.delete(fn); }; }, () => version);
}

/** startLibraryEvents keeps the cached library data current for the whole app session. */
export function startLibraryEvents(): () => void {
  const source = getLibrarySource();
  const stopProgress = source.onSyncProgress((next) => {
    if (next.kind === "favorite" && next.phase === "done" && next.status === "idle") {
      void queryClient.invalidateQueries({ queryKey: ["folders", next.accountId] });
    }
    queryClient.setQueryData<SyncState[]>(queryKeys.syncStates, (old = []) => [...old.filter((s) => s.accountId !== next.accountId || s.kind !== next.kind), next]);
  });
  // A fast sync changes the library many times a second; refresh about once a second.
  let timer: ReturnType<typeof setTimeout> | undefined;
  const stopChanges = source.onLibraryChanged(() => {
    if (timer) return;
    timer = setTimeout(() => {
      timer = undefined;
      version++;
      emit();
      void queryClient.invalidateQueries({ queryKey: ["counts"] });
      void queryClient.invalidateQueries({ queryKey: ["timeline"] });
      void queryClient.invalidateQueries({ queryKey: ["folders"] });
      void queryClient.invalidateQueries({ queryKey: ["action-state"], predicate: (query) => !queryClient.isMutating({ mutationKey: ["work-action", query.queryKey[1], query.queryKey[2]] }) });
    }, 1000);
  });
  const stopAccounts = onAccountsChanged(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.sources });
    void queryClient.invalidateQueries({ queryKey: ["media"] });
    void queryClient.invalidateQueries({ queryKey: queryKeys.syncStates });
    void queryClient.invalidateQueries({ queryKey: ["counts"] });
  });
  return () => { stopProgress(); stopChanges(); stopAccounts(); clearTimeout(timer); };
}
