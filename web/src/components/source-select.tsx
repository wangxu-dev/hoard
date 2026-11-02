import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, LogOut, Plus, Users } from "lucide-react";
import { toast } from "sonner";
import { Accounts } from "@/lib/api";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { SourceIcon } from "@/components/brand/source-icon";
import { setLibraryFilters, useLibraryFilters } from "@/lib/library-filters";
import { getLibrarySource, type SourceAccount, type SourceGroup } from "@/lib/library";

export type AccountSource = "douyin" | "bilibili";
const platforms: AccountSource[] = ["douyin", "bilibili"];
function AccountAvatar({ account, small = false }: { account: SourceAccount; small?: boolean }) {
  return <Avatar className={small ? "size-4.5 shrink-0" : "size-5 shrink-0"}><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>;
}
export function SourceSelect({ sources, onAddAccount, compact = false }: { sources: SourceGroup[]; onAddAccount: (source: AccountSource) => void; compact?: boolean }) {
  const { t } = useTranslation("library");
  const selection = useRef(0);
  const [expanded, setExpanded] = useState(false);
  const pointerInAdd = useRef(false);
  const { scope } = useLibraryFilters();
  const group = sources.find((s) => s.id === scope.source);
  const account = sources.flatMap((s) => s.accounts).find((a) => a.id === scope.accountId);
  const value = scope.accountId ? `account:${scope.accountId}` : scope.source ? `source:${scope.source}` : "all";
  const label = account?.nickname ?? (group ? t("toolbar.source.platform", { name: group.name }) : t("toolbar.source.all"));
  const select = (next: string) => {
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
  };
  return <DropdownMenu onOpenChange={(open) => { if (!open) { pointerInAdd.current = false; setExpanded(false); } }}>
    <Tooltip><TooltipTrigger asChild><DropdownMenuTrigger asChild>
      <Button variant="outline" data-control="source" data-compact={compact} className={`h-8 min-w-0 shrink-0 justify-between font-normal text-[13px] ${compact ? "w-12 gap-1 px-1" : "w-45 px-3"}`} aria-label={`${t("toolbar.source.label")}: ${label}`}>
        <span className="flex min-w-0 items-center gap-2">{account ? <AccountAvatar account={account} small /> : group ? <SourceIcon source={group.id} /> : <Users className="size-3.5" />}{!compact && <span className="truncate">{label}</span>}</span><ChevronDown className="size-4 shrink-0 opacity-50" />
      </Button>
    </DropdownMenuTrigger></TooltipTrigger><TooltipContent>{label}</TooltipContent></Tooltip>
    <DropdownMenuContent align="start" className="w-64" onCloseAutoFocus={() => setExpanded(false)}>
      <DropdownMenuRadioGroup value={value} onValueChange={select}>
        <DropdownMenuRadioItem value="all" className="h-8 text-[13px]"><Users className="size-3.5" />{t("toolbar.source.all")}</DropdownMenuRadioItem>
        {sources.map((source) => <div key={source.id}>
          <DropdownMenuLabel className="flex items-center gap-2 text-xs text-muted-foreground"><SourceIcon source={source.id} />{source.name}</DropdownMenuLabel>
          {source.accounts.length >= 2 && <DropdownMenuRadioItem value={`source:${source.id}`} className="h-8 text-[13px]"><SourceIcon source={source.id} />{t("toolbar.source.platform", { name: source.name })}</DropdownMenuRadioItem>}
          {source.accounts.map((a) => <DropdownMenuRadioItem key={a.id} value={`account:${a.id}`} className="h-8 min-w-0 text-[13px]">
            <AccountAvatar account={a} /><span className="min-w-0 flex-1 truncate">{a.nickname}</span>
            {a.status === "signed_out" ? <span role="img" aria-label={t("toolbar.source.signed_out")} className="ml-auto size-1.5 shrink-0 rounded-full bg-destructive" /> : a.syncing ? <span role="img" aria-label={t("toolbar.source.syncing")} className="source-pulse ml-auto size-1.5 shrink-0 rounded-full bg-primary" /> : null}
          </DropdownMenuRadioItem>)}
        </div>)}
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />
      <div data-testid="add-account-row" data-expanded={expanded} className="flex h-10 items-center gap-1 px-1" onPointerEnter={() => { pointerInAdd.current = true; setExpanded(true); }} onPointerLeave={(event) => { pointerInAdd.current = false; if (!event.currentTarget.contains(document.activeElement)) setExpanded(false); }} onBlur={(event) => { if (!pointerInAdd.current && !event.currentTarget.contains(event.relatedTarget)) setExpanded(false); }}>
        <DropdownMenuItem className="h-8 min-w-0 flex-1 text-[13px]" onFocus={() => setExpanded(true)} onSelect={(event) => { event.preventDefault(); setExpanded(true); }} aria-expanded={expanded}>
          <Plus className="size-3.5" /><span>{t("toolbar.source.add")}</span>
        </DropdownMenuItem>
        <div className="flex w-17 shrink-0 items-center gap-1">
          {platforms.map((source, index) => <DropdownMenuItem key={source} aria-label={t("toolbar.source.addPlatform", { name: t(`toolbar.source.${source}`) })} onFocus={() => setExpanded(true)} onSelect={() => onAddAccount(source)}
              className="size-8 shrink-0 justify-center rounded-full bg-transparent p-0 focus:bg-transparent [&_svg]:text-foreground">
              <span data-platform-icon className={`pointer-events-none flex size-8 items-center justify-center rounded-full bg-popover transition-transform duration-(--duration-slow) ease-(--ease-out) motion-reduce:transition-none ${index === 1 && !expanded ? "-translate-x-7" : "translate-x-0"}`}><SourceIcon source={source} className="size-4.5" /></span>
            </DropdownMenuItem>)}
        </div>
      </div>
      {account?.status === "active" && <DropdownMenuItem className="h-8 text-[13px]" onSelect={() => { void Accounts.signOut(account.id).catch(() => toast.error(t("toolbar.source.signOutFailed"))); }}><LogOut className="size-3.5" />{t("toolbar.source.signOut")}</DropdownMenuItem>}
    </DropdownMenuContent>
  </DropdownMenu>;
}
