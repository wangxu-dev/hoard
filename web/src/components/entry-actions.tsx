import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Heart, Star, Music2, MoreHorizontal, Check } from "lucide-react";
import { motion } from "motion/react";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem } from "@/components/ui/context-menu";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { getLibrarySource, type LibraryEntry } from "@/lib/library";
import { useLibraryFilters } from "@/lib/library-filters";
import { usePreviewOwners, setPreviewRelation, type ActionTarget, type ActionKind } from "@/lib/action-preview";
import { fixtureName } from "@/lib/demo";
import { queryKeys } from "@/lib/query";
import { fastTransition } from "@/lib/motion";

function AccountChoices({ target }: { target: ActionTarget }) {
  const { t } = useTranslation("library");
  const owners = usePreviewOwners(target);
  const { scope } = useLibraryFilters();
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources() });
  const accounts = data?.flatMap((group) => group.accounts).filter((account) => !scope.accountId || account.id === scope.accountId) ?? [];
  return <div className="space-y-1" data-account-choices>
    {accounts.map((account) => {
      const on = owners.includes(account.id);
      return <Button key={account.id} variant="ghost" className="h-auto w-full justify-start gap-3 px-2 py-2" aria-label={t(`actions.${on ? "remove" : "add"}.${target.kind}`) + " · " + account.nickname}
        onClick={() => setPreviewRelation(target, account.id, !on)}>
        <Avatar className="size-8"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
        <span className="min-w-0 flex-1 truncate text-left">{account.nickname}</span>
        <span className="text-xs text-muted-foreground">{t(`actions.${on ? "remove" : "add"}.${target.kind}`)}</span>
        <motion.span key={String(on)} initial={{ scale: 0.8 }} animate={{ scale: 1 }} transition={fastTransition} className="w-4 text-primary">{on && <Check className="size-4" />}</motion.span>
      </Button>;
    })}
  </div>;
}

function ActionButton({ target }: { target: ActionTarget }) {
  const { t } = useTranslation("library");
  const [open, setOpen] = useState(false);
  const { scope } = useLibraryFilters();
  const owners = usePreviewOwners(target);
  const Icon = target.kind === "like" ? Heart : target.kind === "favorite" ? Star : Music2;
  const on = scope.accountId ? owners.includes(scope.accountId) : owners.length > 0;
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild>
      <Button variant="secondary" aria-label={t(`actions.label.${target.kind}`)} onPointerEnter={() => setOpen(true)}
        onClick={(event) => { if (scope.accountId) { event.preventDefault(); setPreviewRelation(target, scope.accountId, !on); } }}>
        <motion.span key={String(on)} initial={{ scale: 0.8 }} animate={{ scale: 1 }} transition={fastTransition}><Icon className={on ? "fill-current text-primary" : ""} /></motion.span>
        {t(`actions.label.${target.kind}`)}
        {!scope.accountId && <span className="text-xs tabular-nums">{owners.length}</span>}
      </Button>
    </PopoverTrigger>
    <PopoverContent side="left" className="dark z-[60] w-80 max-w-[calc(100vw-2rem)] p-3" onOpenAutoFocus={(event) => event.preventDefault()} onPointerLeave={() => setOpen(false)}>
      <p className="mb-2 text-sm font-medium">{t("actions.choose")}</p>
      <AccountChoices target={target} />
    </PopoverContent>
  </Popover>;
}

export function PreviewActions({ entry }: { entry: LibraryEntry }) {
  const { t } = useTranslation("library");
  if (!fixtureName) return null;
  return <div className="space-y-3 border-t border-viewer-border p-6" data-preview-actions>
    <p className="text-xs text-viewer-muted">{t("actions.preview")}</p>
    <div className="flex flex-wrap gap-2">
      <ActionButton target={{ id: entry.id, kind: "like" }} />
      <ActionButton target={{ id: entry.id, kind: "favorite" }} />
      <ActionButton target={{ id: `demo-music-${Number(entry.id.match(/\d+$/)?.[0] ?? 0) % 78}`, kind: "music" }} />
    </div>
  </div>;
}

export function EntryActionMenu({ entry, children }: { entry: LibraryEntry; children: ReactNode }) {
  const { t } = useTranslation("library");
  const [action, setAction] = useState<ActionKind | null>(null);
  if (!fixtureName) return <>{children}</>;
  const kinds: ActionKind[] = entry.kind === "music" ? ["music"] : ["like", "favorite"];
  return <ContextMenu>
    <ContextMenuTrigger asChild><div className="relative min-w-0">{children}
      <div className="absolute right-1 bottom-1" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
        <DropdownMenu><DropdownMenuTrigger asChild><Button variant="secondary" size="icon" className="size-6" aria-label={t("actions.more")}><MoreHorizontal /></Button></DropdownMenuTrigger>
          <DropdownMenuContent>{kinds.map((kind) => <DropdownMenuItem key={kind} onSelect={() => setAction(kind)}>{t(`actions.label.${kind}`)}</DropdownMenuItem>)}</DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div></ContextMenuTrigger>
    <ContextMenuContent>{kinds.map((kind) => <ContextMenuItem key={kind} onSelect={() => setAction(kind)}>{t(`actions.label.${kind}`)}</ContextMenuItem>)}</ContextMenuContent>
    <Dialog open={action !== null} onOpenChange={(open) => { if (!open) setAction(null); }}>
      <DialogContent className="sm:max-w-sm">
        <DialogTitle>{t("actions.choose")}</DialogTitle><DialogDescription>{entry.title} · {t("actions.preview")}</DialogDescription>
        {action && <AccountChoices target={{ id: entry.id, kind: action }} />}
      </DialogContent>
    </Dialog>
  </ContextMenu>;
}
