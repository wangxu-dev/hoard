import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { getLibrarySource, type LibraryEntry } from "@/lib/library";
import { usePreviewOwners } from "@/lib/action-preview";
import { useLibraryFilters } from "@/lib/library-filters";
import { fixtureName } from "@/lib/demo";
import { queryKeys } from "@/lib/query";

export function EntryAccounts({ entry }: { entry: LibraryEntry }) {
  const { t } = useTranslation("library");
  const [open, setOpen] = useState(false);
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources(), staleTime: 60_000 });
  const { scope, kind } = useLibraryFilters();
  const preview = usePreviewOwners({ id: entry.id, kind });
  const ownerIds = fixtureName ? preview.filter((id) => !scope.accountId || id === scope.accountId) : entry.accountIds;
  const accounts = data?.flatMap((source) => source.accounts).filter((account) => ownerIds?.includes(account.id)) ?? [];
  if (!accounts.length) return null;
  return <div data-entry-accounts onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="secondary" size="sm" aria-label={t("accounts.owners", { count: accounts.length })}
          onPointerEnter={() => setOpen(true)} className="h-auto rounded-full p-1 shadow-sm">
          <span className="flex -space-x-2">
            {accounts.slice(0, 3).map((account) => <Avatar key={account.id} className="size-6 ring-2 ring-background">
              <AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback className="text-xs">{account.nickname.slice(0, 1)}</AvatarFallback>
            </Avatar>)}
          </span>
          {accounts.length > 3 && <span className="px-1 text-xs">+{accounts.length - 3}</span>}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-64 p-2" onOpenAutoFocus={(event) => event.preventDefault()} onPointerLeave={() => setOpen(false)}>
        <p className="px-2 py-1 text-xs text-muted-foreground">{t("accounts.owners", { count: accounts.length })}</p>
        {accounts.map((account) => <div key={account.id} className="flex min-w-0 items-center gap-2 p-2">
          <Avatar className="size-8"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
          <span className="min-w-0 truncate text-sm">{account.nickname}</span>
        </div>)}
      </PopoverContent>
    </Popover>
  </div>;
}
