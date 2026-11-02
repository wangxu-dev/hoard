import { useRef } from "react";
import { useTranslation } from "react-i18next";
import { LogOut, Plus, Users } from "lucide-react";
import { toast } from "sonner";
import { Accounts } from "@/lib/api";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectSeparator, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { SourceIcon } from "@/components/brand/source-icon";
import { setLibraryFilters, useLibraryFilters } from "@/lib/library-filters";
import { getLibrarySource, type SourceAccount, type SourceGroup } from "@/lib/library";

function AccountAvatar({ account, small = false }: { account: SourceAccount; small?: boolean }) {
  return <Avatar className={small ? "size-4.5 shrink-0" : "size-5 shrink-0"}><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>;
}
export function SourceSelect({ sources, onAddAccount, compact = false }: { sources: SourceGroup[]; onAddAccount: () => void; compact?: boolean }) {
  const { t } = useTranslation("library");
  const selection = useRef(0);
  const { scope } = useLibraryFilters();
  const group = sources.find((s) => s.id === scope.source);
  const account = sources.flatMap((s) => s.accounts).find((a) => a.id === scope.accountId);
  const value = scope.accountId ? `account:${scope.accountId}` : scope.source ? `source:${scope.source}` : "all";
  const label = account?.nickname ?? (group ? t("toolbar.source.platform", { name: group.name }) : t("toolbar.source.all"));
  return <Select value={value} onValueChange={(next) => {
    if (next === "add") { onAddAccount(); return; }
    // Signing out forgets this account's login in Hoard only; its synced items stay.
    if (next === "signout") { if (account) void Accounts.signOut(account.id).catch(() => toast.error(t("toolbar.source.signOutFailed"))); return; }
    const selected = sources.flatMap((s) => s.accounts).find((a) => `account:${a.id}` === next);
    const request = ++selection.current;
    setLibraryFilters({ scope: selected ? { source: selected.source, accountId: selected.id } : next === "all" ? {} : { source: next.slice(7) } });
    if (selected?.status === "active" && selected.id !== scope.accountId) {
      const source = getLibrarySource();
      void source.syncStates().then((states) => {
        if (request !== selection.current) return;
        const lastSync = Math.max(0, ...states.filter((s) => s.accountId === selected.id && s.kind !== "cover").map((s) => s.lastSyncAt ?? 0));
        if (!lastSync || Date.now() / 1000 - lastSync > 180) return source.startSync(selected.id);
      }).catch(() => toast.error(t("sync.startFailed")));
    }
  }}>
    <Tooltip><TooltipTrigger asChild>
    <SelectTrigger size="sm" data-control="source" data-compact={compact} className={`h-8 min-w-0 shrink-0 text-[13px] ${compact ? "w-12 gap-1 px-1" : "w-45"}`} aria-label={`${t("toolbar.source.label")}: ${label}`}>
      <SelectValue><span className="flex min-w-0 items-center gap-2">
        {account ? <AccountAvatar account={account} small /> : group ? <SourceIcon source={group.id} /> : <Users className="size-3.5" />}
        {!compact && <span className="truncate">{label}</span>}
      </span></SelectValue>
    </SelectTrigger>
    </TooltipTrigger><TooltipContent>{label}</TooltipContent></Tooltip>
    <SelectContent position="popper" align="start" className="w-64">
      <SelectItem value="all" className="h-8 text-[13px]"><Users className="size-3.5" />{t("toolbar.source.all")}</SelectItem>
      {sources.map((source) => <SelectGroup key={source.id}>
        <SelectLabel className="flex items-center gap-2"><SourceIcon source={source.id} />{source.name}</SelectLabel>
        {source.accounts.length >= 2 && <SelectItem value={`source:${source.id}`} className="h-8 text-[13px]"><SourceIcon source={source.id} />{t("toolbar.source.platform", { name: source.name })}</SelectItem>}
        {source.accounts.map((a) => <SelectItem key={a.id} value={`account:${a.id}`} className="h-8 text-[13px] [&>span:last-child]:w-full [&>span:last-child]:min-w-0">
          <AccountAvatar account={a} /><span className="min-w-0 flex-1 truncate">{a.nickname}</span>
          {a.status === "signed_out" ? <span role="img" aria-label={t("toolbar.source.signed_out")} className="ml-auto size-1.5 shrink-0 rounded-full bg-destructive" /> : a.syncing ? <span role="img" aria-label={t("toolbar.source.syncing")} className="source-pulse ml-auto size-1.5 shrink-0 rounded-full bg-primary" /> : null}
        </SelectItem>)}
      </SelectGroup>)}
      <SelectSeparator /><SelectItem value="add" className="h-8 text-[13px]"><Plus className="size-3.5" />{t("toolbar.source.add")}</SelectItem>
      {account?.status === "active" && <SelectItem value="signout" className="h-8 text-[13px]"><LogOut className="size-3.5" />{t("toolbar.source.signOut")}</SelectItem>}
    </SelectContent>
  </Select>;
}
