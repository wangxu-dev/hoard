import { useQuery } from "@tanstack/react-query";
import { ChevronDown, Users } from "lucide-react";
import { useTranslation } from "react-i18next";
import { SourceIcon } from "@/components/brand/source-icon";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { getLibrarySource, type SourceAccount } from "@/lib/library";
import { setLibraryFilters, useLibraryFilters } from "@/lib/library-filters";
import { queryKeys } from "@/lib/query";

function AccountAvatar({ account, small = false }: { account: SourceAccount; small?: boolean }) {
  return (
    <Avatar className={small ? "size-4.5 shrink-0" : "size-5 shrink-0"}>
      <AvatarImage src={account.avatarUrl} alt="" />
      <AvatarFallback className="text-[10px]">{account.nickname.slice(0, 1)}</AvatarFallback>
    </Avatar>
  );
}

/** Glass-styled account scope selector shared with the library filters. */
export function ExploreScopeSelect() {
  const { t } = useTranslation("library");
  const { data: sources = [] } = useQuery({
    queryKey: queryKeys.sources,
    queryFn: () => getLibrarySource().sources(),
    staleTime: 60_000,
  });
  const { scope } = useLibraryFilters();

  const totalAccounts = sources.reduce((count, source) => count + source.accounts.length, 0);
  if (totalAccounts <= 1) return null;

  const group = sources.find((s) => s.id === scope.source);
  const account = sources.flatMap((s) => s.accounts).find((a) => a.id === scope.accountId);
  const value = scope.accountId ? `account:${scope.accountId}` : scope.source ? `source:${scope.source}` : "all";
  const label = account?.nickname ?? (group ? t("toolbar.source.platform", { name: group.name }) : t("toolbar.source.all"));

  const select = (next: string) => {
    const selected = sources.flatMap((s) => s.accounts).find((a) => `account:${a.id}` === next);
    setLibraryFilters({
      scope: selected ? { source: selected.source, accountId: selected.id } : next === "all" ? {} : { source: next.slice(7) },
    });
  };

  return (
    <>
      <span className="explore-nav-divider" />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            data-control="explore-scope"
            className="max-w-44 text-[13px] font-normal"
            aria-label={`${t("toolbar.source.label")}: ${label}`}
            title={label}
          >
            {account ? (
              <AccountAvatar account={account} small />
            ) : group ? (
              <SourceIcon source={group.id} className="size-3.5 shrink-0" />
            ) : (
              <Users className="size-3.5 shrink-0" />
            )}
            <span className="min-w-0 truncate">{label}</span>
            <ChevronDown className="size-3.5 shrink-0 opacity-60" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" sideOffset={8} className="dark explore-glass w-60 rounded-xl">
          <DropdownMenuRadioGroup value={value} onValueChange={select}>
            <DropdownMenuRadioItem value="all" className="h-8 text-[13px]">
              <Users className="size-3.5" />
              {t("toolbar.source.all")}
            </DropdownMenuRadioItem>
            {sources.map((source) => (
              <div key={source.id}>
                <DropdownMenuLabel className="flex items-center gap-2 text-xs text-muted-foreground">
                  <SourceIcon source={source.id} />
                  {source.name}
                </DropdownMenuLabel>
                {source.accounts.length >= 2 && (
                  <DropdownMenuRadioItem value={`source:${source.id}`} className="h-8 text-[13px]">
                    <SourceIcon source={source.id} />
                    {t("toolbar.source.platform", { name: source.name })}
                  </DropdownMenuRadioItem>
                )}
                {source.accounts.map((a) => (
                  <DropdownMenuRadioItem key={a.id} value={`account:${a.id}`} className="h-8 min-w-0 text-[13px]">
                    <AccountAvatar account={a} />
                    <span className="min-w-0 flex-1 truncate">{a.nickname}</span>
                    {a.status === "signed_out" ? (
                      <span role="img" aria-label={t("toolbar.source.signed_out")} className="ml-auto size-1.5 shrink-0 rounded-full bg-destructive" />
                    ) : a.syncing ? (
                      <span role="img" aria-label={t("toolbar.source.syncing")} className="source-pulse ml-auto size-1.5 shrink-0 rounded-full bg-primary" />
                    ) : null}
                  </DropdownMenuRadioItem>
                ))}
              </div>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}
