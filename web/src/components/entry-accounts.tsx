import { createContext, memo, useCallback, useContext, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import type { LibraryEntry, ListKind, Scope, SourceGroup } from "@/lib/library";
import { usePreviewOwners } from "@/lib/action-preview";
import { fixtureName } from "@/lib/demo";
import type { ActionState } from "@/lib/api";
import { actionField, actionKey } from "@/lib/work-actions";
import { queryClient } from "@/lib/query";

export const EntryAccountsContext = createContext<{ sources: SourceGroup[]; scope: Scope; kind: ListKind }>({ sources: [], scope: {}, kind: "like" });

function RealOwners({ entry }: { entry: LibraryEntry }) {
  const { sources, scope, kind } = useContext(EntryAccountsContext);
  const accounts = sources.find((group) => group.id === (entry.source ?? "douyin"))?.accounts ?? [];
  const snapshot = () => accounts.filter((account) => {
    if (scope.accountId && scope.accountId !== account.id) return false;
    const state = queryClient.getQueryData<ActionState>(actionKey(account.id, entry.id));
    return kind !== "music" && state ? state[actionField(kind)] : entry.accountIds?.includes(account.id);
  }).map((account) => account.id).join(",");
  const subscribe = useCallback((notify: () => void) => queryClient.getQueryCache().subscribe((event) => {
    if (event.type === "updated" && event.query.queryKey[0] === "action-state" && event.query.queryKey[2] === entry.id) notify();
  }), [entry.id]);
  const owners = useSyncExternalStore(subscribe, snapshot);
  return <Owners ids={owners ? owners.split(",") : []} />;
}

function PreviewOwners({ entry }: { entry: LibraryEntry }) {
  const { kind } = useContext(EntryAccountsContext);
  const owners = usePreviewOwners({ id: entry.id, source: entry.source, kind });
  return <Owners ids={owners} />;
}

function Owners({ ids }: { ids: readonly string[] }) {
  const { t } = useTranslation("library");
  const { sources, scope } = useContext(EntryAccountsContext);
  const accounts = sources.flatMap((source) => source.accounts).filter((account) => ids.includes(account.id) && (!scope.accountId || scope.accountId === account.id));
  if (!accounts.length) return null;
  return <div data-entry-accounts role="group" tabIndex={0}
    aria-label={`${t("accounts.owners", { count: accounts.length })}: ${accounts.map((account) => account.nickname).join(", ")}`}
    onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}
    className="group/owners flex flex-row-reverse items-center rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring">
    {accounts.map((account) => <Avatar key={account.id} aria-label={account.nickname}
      className="size-6 bg-transparent transition-[margin] duration-(--duration-slow) ease-(--ease-out) motion-reduce:transition-none not-last:-ml-[1.125rem] group-hover/entry:not-last:ml-1 group-focus-within/entry:not-last:ml-1 after:hidden">
      <AvatarImage src={account.avatarUrl} alt={account.nickname} />
      <AvatarFallback className="text-xs">{account.nickname.slice(0, 1)}</AvatarFallback>
    </Avatar>)}
  </div>;
}

export const EntryAccounts = memo(function EntryAccounts({ entry }: { entry: LibraryEntry }) {
  return fixtureName ? <PreviewOwners entry={entry} /> : <RealOwners entry={entry} />;
});
