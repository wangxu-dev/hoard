import { Actions, Accounts, App, Exploration, Library, Player, Sync, Updates, events } from "@mygo";
import type { Account, LibraryChange, SyncView } from "@mygo";

export { Actions, Accounts, App, Exploration, Library, Player, Sync, Updates, events };
export type { ExploreScope, ExplorationDiagnostic, Work as ExplorationWork, ActionState, ActionFolder, LibraryFolder, Account, LibraryItem, LibraryPage, LibraryQuery, LikeItem, LikesResult, LoginState, Status, SyncView, UpdateDownload, UpdateInfo } from "@mygo";

export function onAccountsChanged(listener: (accounts: Account[]) => void): () => void {
  return events.accountsChanged.on(listener);
}
export function onSyncProgress(listener: (view: SyncView) => void): () => void {
  return events.syncProgress.on(listener);
}
export function onLibraryChanged(listener: (change: LibraryChange) => void): () => void {
  return events.libraryChanged.on(listener);
}
