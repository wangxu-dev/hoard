export interface ExploreScopeIdentity {
  accountId: string;
  source: string;
}

export interface ScopeCacheEntry<TSnapshot, TModel> {
  snapshot: TSnapshot;
  model: TModel;
  key: string;
}

/** Small in-memory LRU cache keyed by Explore scope (`accountId:source`). */
export class ExploreScopeCache<TSnapshot, TModel> {
  private readonly entries = new Map<string, ScopeCacheEntry<TSnapshot, TModel>>();
  private readonly maxEntries: number;

  constructor(maxEntries = 4) {
    this.maxEntries = Math.max(0, maxEntries);
  }

  get(scopeKey: string): ScopeCacheEntry<TSnapshot, TModel> | undefined {
    const entry = this.entries.get(scopeKey);
    if (!entry) return undefined;
    this.entries.delete(scopeKey);
    this.entries.set(scopeKey, entry);
    return entry;
  }

  set(scopeKey: string, entry: ScopeCacheEntry<TSnapshot, TModel>): void {
    if (this.maxEntries === 0) return;
    if (this.entries.has(scopeKey)) {
      this.entries.delete(scopeKey);
    }
    this.entries.set(scopeKey, entry);
    while (this.entries.size > this.maxEntries) {
      const oldestKey = this.entries.keys().next().value;
      if (oldestKey === undefined) break;
      this.entries.delete(oldestKey);
    }
  }

  clear(): void {
    this.entries.clear();
  }
}

/** Returns true when a cached snapshot matches the completed native task status and scope. */
export function canReuseSnapshot(
  status: { inputRevision?: string; snapshotId?: string },
  cached:
    | {
        schemaVersion?: number;
        inputRevision?: string;
        snapshotId?: string;
        scope: ExploreScopeIdentity;
      }
    | null
    | undefined,
  scope: ExploreScopeIdentity,
): boolean {
  if (!cached) return false;
  if (cached.schemaVersion !== 3) return false;
  if (cached.scope.accountId !== scope.accountId || cached.scope.source !== scope.source) return false;
  if (!status.inputRevision || status.inputRevision !== cached.inputRevision) return false;
  if (status.snapshotId && cached.snapshotId && status.snapshotId !== cached.snapshotId) return false;
  return true;
}

/** Returns true when a library change for `changeAccountId` can affect the active Explore scope. */
export function libraryChangeAffectsScope(
  changeAccountId: string,
  scope: ExploreScopeIdentity,
): boolean {
  if (scope.accountId && changeAccountId && changeAccountId !== scope.accountId) return false;
  return true;
}
