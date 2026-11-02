import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { getLibrarySource, type LibraryEntry } from "@/lib/library";
import { usePreviewOwners } from "@/lib/action-preview";
import { useLibraryFilters } from "@/lib/library-filters";
import { fixtureName } from "@/lib/demo";
import { queryKeys } from "@/lib/query";

export function EntryAccounts({ entry }: { entry: LibraryEntry }) {
  const { t } = useTranslation("library");
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources(), staleTime: 60_000 });
  const { scope, kind } = useLibraryFilters();
  const preview = usePreviewOwners({ id: entry.id, kind });
  const ownerIds = fixtureName ? preview.filter((id) => !scope.accountId || id === scope.accountId) : entry.accountIds;
  const accounts = data?.flatMap((source) => source.accounts).filter((account) => ownerIds?.includes(account.id)) ?? [];
  if (!accounts.length) return null;
  return <div data-entry-accounts role="group" tabIndex={0}
    aria-label={`${t("accounts.owners", { count: accounts.length })}: ${accounts.map((account) => account.nickname).join(", ")}`}
    onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}
    className="group/owners flex flex-row-reverse items-center rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring">
    {accounts.map((account) => <Avatar key={account.id} aria-label={account.nickname}
      className="size-6 bg-transparent transition-[margin] duration-slow ease-(--ease-out) motion-reduce:transition-none not-last:-ml-[1.125rem] group-hover/entry:not-last:ml-1 group-focus-within/entry:not-last:ml-1 after:hidden">
      <AvatarImage src={account.avatarUrl} alt={account.nickname} />
      <AvatarFallback className="text-xs">{account.nickname.slice(0, 1)}</AvatarFallback>
    </Avatar>)}
  </div>;
}
