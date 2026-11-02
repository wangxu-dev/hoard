import { useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
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
import { Actions, type ActionState } from "@/lib/api";
import { toast } from "sonner";
import { queryClient, queryKeys } from "@/lib/query";
import { fastTransition } from "@/lib/motion";

function RealAccountChoice({ target, account, compact = false }: { target: ActionTarget; compact?: boolean; account: { id: string; nickname: string; avatarUrl: string; status: string } }) {
  const { t } = useTranslation("library");
  const key = ["action-state", account.id, target.id];
  const state = useQuery({ queryKey: key, queryFn: () => Actions.state(account.id, target.id), enabled: account.status === "active", staleTime: 15_000, retry: false });
  const mutation = useMutation({
    mutationFn: (on: boolean) => Actions.set(account.id, target.id, target.kind, on),
    onSuccess: (result: ActionState) => {
      queryClient.setQueryData(key, result);
      void queryClient.invalidateQueries({ queryKey: ["counts"] });
      toast.success(t("actions.saved"));
    },
    onError: (error: Error) => {
      void queryClient.invalidateQueries({ queryKey: key });
      toast.error(t(error.message.includes("ACTION_UNKNOWN") ? "actions.unknown" : "actions.failed"));
    },
  });
  const on = target.kind === "like" ? state.data?.liked : state.data?.favorited;
  const unavailable = account.status !== "active";
  const Icon = target.kind === "like" ? Heart : Star;
  if (compact) return <Button variant="ghost" size="icon" aria-label={t(`actions.${on ? "remove" : "add"}.${target.kind}`)} aria-pressed={!!on}
    disabled={unavailable || state.isFetching || mutation.isPending || on === undefined} onClick={() => mutation.mutate(!on)}>
    <Icon className={on ? "fill-current text-primary" : ""} />
  </Button>;
  return <Button variant="ghost" className="h-auto w-full justify-start gap-3 px-2 py-2" disabled={unavailable || state.isFetching || mutation.isPending || on === undefined}
    onClick={() => { if (on !== undefined) mutation.mutate(!on); }}>
    <Avatar className="size-8"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
    <span className="min-w-0 flex-1 truncate text-left">{account.nickname}</span>
    <span className="text-xs text-muted-foreground">{t(unavailable ? "actions.login" : mutation.isPending ? "actions.saving" : state.isFetching ? "actions.loading" : state.isError ? "actions.failed" : `actions.${on ? "remove" : "add"}.${target.kind}`)}</span>
  </Button>;
}

function RealAccountChoices({ target }: { target: ActionTarget }) {
  const { t } = useTranslation("library");
  const { scope } = useLibraryFilters();
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources() });
  if (target.kind === "music") return <p className="text-sm text-muted-foreground">{t("actions.musicUnavailable")}</p>;
  return <div data-account-choices>{data?.filter((group) => group.id === "douyin").flatMap((group) => group.accounts).filter((account) => !scope.accountId || account.id === scope.accountId).map((account) => <RealAccountChoice key={account.id} target={target} account={account} />)}</div>;
}

function AccountChoices({ target }: { target: ActionTarget }) {
  return fixtureName ? <PreviewAccountChoices target={target} /> : <RealAccountChoices target={target} />;
}

function PreviewAccountChoices({ target }: { target: ActionTarget }) {
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
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources() });
  const accounts = data?.filter((group) => group.id === "douyin").flatMap((group) => group.accounts).filter((account) => !scope.accountId || account.id === scope.accountId) ?? [];
  const single = accounts.length === 1 ? accounts[0] : undefined;
  const Icon = target.kind === "like" ? Heart : target.kind === "favorite" ? Star : Music2;
  const on = !!fixtureName && (single ? owners.includes(single.id) : owners.length > 0);
  if (single && !fixtureName) return <RealAccountChoice target={target} account={single} compact />;
  if (single) return <Button variant="ghost" size="icon" aria-label={t(`actions.label.${target.kind}`)} aria-pressed={on}
    onClick={() => setPreviewRelation(target, single.id, !on)}><Icon className={on ? "fill-current text-primary" : ""} /></Button>;
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild>
      <Button variant="ghost" size="icon" disabled={!accounts.length} aria-label={t(`actions.label.${target.kind}`)}>
        <motion.span key={String(on)} initial={{ scale: 0.8 }} animate={{ scale: 1 }} transition={fastTransition}><Icon className={on ? "fill-current text-primary" : ""} /></motion.span>
      </Button>
    </PopoverTrigger>
    <PopoverContent side="top" align="end" className="dark z-[60] w-72 max-w-[calc(100vw-2rem)] p-2">
      <AccountChoices target={target} />
    </PopoverContent>
  </Popover>;
}

export function PreviewActions({ entry }: { entry: LibraryEntry }) {
  const { t } = useTranslation("library");
  return <div className="space-y-3 border-t border-viewer-border p-6" data-preview-actions>
    {fixtureName && <p className="text-xs text-viewer-muted">{t("actions.preview")}</p>}
    <div className="flex flex-wrap gap-2">
      <ActionButton target={{ id: entry.id, kind: "like" }} />
      <ActionButton target={{ id: entry.id, kind: "favorite" }} />
      {fixtureName && <ActionButton target={{ id: `demo-music-${Number(entry.id.match(/\d+$/)?.[0] ?? 0) % 78}`, kind: "music" }} />}
    </div>
  </div>;
}

export function EntryActionMenu({ entry, children }: { entry: LibraryEntry; children: ReactNode }) {
  const { t } = useTranslation("library");
  const [action, setAction] = useState<ActionKind | null>(null);
  if (!fixtureName && entry.kind === "music") return <>{children}</>;
  const kinds: ActionKind[] = entry.kind === "music" ? ["music"] : ["like", "favorite"];
  return <ContextMenu>
    <ContextMenuTrigger asChild><div className="group/card-menu relative min-w-0">{children}
      {entry.kind === "music" && <div className="absolute right-1.5 top-1.5 opacity-0 transition-opacity duration-base group-hover/card-menu:opacity-100 group-focus-within/card-menu:opacity-100 motion-reduce:transition-none" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
        <DropdownMenu><DropdownMenuTrigger asChild><Button variant="secondary" size="icon" className="size-6" aria-label={t("actions.more")}><MoreHorizontal /></Button></DropdownMenuTrigger>
          <DropdownMenuContent>{kinds.map((kind) => <DropdownMenuItem key={kind} onSelect={() => setAction(kind)}>{kind === "like" ? <Heart /> : kind === "favorite" ? <Star /> : <Music2 />}{t(`actions.label.${kind}`)}</DropdownMenuItem>)}</DropdownMenuContent>
        </DropdownMenu>
      </div>}
    </div></ContextMenuTrigger>
    <ContextMenuContent>{kinds.map((kind) => <ContextMenuItem key={kind} onSelect={() => setAction(kind)}>{kind === "like" ? <Heart /> : kind === "favorite" ? <Star /> : <Music2 />}{t(`actions.label.${kind}`)}</ContextMenuItem>)}</ContextMenuContent>
    <Dialog open={action !== null} onOpenChange={(open) => { if (!open) setAction(null); }}>
      <DialogContent className="sm:max-w-sm">
        <DialogTitle>{t("actions.choose")}</DialogTitle><DialogDescription>{entry.title}{fixtureName ? ` · ${t("actions.preview")}` : ""}</DialogDescription>
        {action && <AccountChoices target={{ id: entry.id, kind: action }} />}
      </DialogContent>
    </Dialog>
  </ContextMenu>;
}
