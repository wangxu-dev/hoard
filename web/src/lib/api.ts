import { Accounts, App, Library, Player, Sync, Updates, events } from "@mygo";
import type { Account, LibraryChange, SyncView } from "@mygo";

export { Accounts, App, Library, Player, Sync, Updates, events };
export type { Account, LibraryItem, LibraryPage, LibraryQuery, LikeItem, LikesResult, LoginState, Status, SyncView, UpdateInfo } from "@mygo";

export function onAccountsChanged(listener: (accounts: Account[]) => void): () => void {
  return events.accountsChanged.on(listener);
}
export function onSyncProgress(listener: (view: SyncView) => void): () => void {
  return events.syncProgress.on(listener);
}
export function onLibraryChanged(listener: (change: LibraryChange) => void): () => void {
  return events.libraryChanged.on(listener);
}
