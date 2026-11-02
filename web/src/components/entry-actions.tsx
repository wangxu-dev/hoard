import { useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Heart, Star, Music2, MoreHorizontal, Check } from "lucide-react";
import { motion } from "motion/react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ControlButton } from "@/components/viewer/controls";
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

const activeColor = (kind: ActionKind) => kind === "like" ? "text-(--action-like)" : "text-(--action-favorite)";

function RealAccountChoice({ target, account, compact = false }: { target: ActionTarget; compact?: boolean; account: { id: string; nickname: string; avatarUrl: string; status: string } }) {
  const { t } = useTranslation("library");
  const key = ["action-state", account.id, target.id];
  const initial = { liked: target.likedAccountIds?.includes(account.id) ?? false, favorited: target.favoritedAccountIds?.includes(account.id) ?? false };
  const state = useQuery({ queryKey: key, queryFn: () => Actions.state(account.id, target.id), initialData: initial, enabled: false });
  const mutation = useMutation({
    mutationKey: ["work-action", account.id, target.id, target.kind],
    mutationFn: (on: boolean) => Actions.set(account.id, target.id, target.kind, on),
    onMutate: async (next: boolean) => {
      await queryClient.cancelQueries({ queryKey: key });
      const field = target.kind === "like" ? "liked" : "favorited";
      const before = queryClient.getQueryData<ActionState>(key)?.[field] ?? initial[field];
      queryClient.setQueryData<ActionState>(key, (old = initial) => ({ ...old, [field]: next }));
      return { field, before };
    },
    onSuccess: (result: ActionState) => {
      const field = target.kind === "like" ? "liked" : "favorited";
      queryClient.setQueryData<ActionState>(key, (old = initial) => ({ ...old, [field]: result[field] }));
      void queryClient.invalidateQueries({ queryKey: ["counts"] });
    },
    onError: (error: Error, _next, context) => {
      if (context) queryClient.setQueryData<ActionState>(key, (old = initial) => ({ ...old, [context.field]: context.before }));
      toast.error(t(error.message.includes("ACTION_UNKNOWN") ? "actions.unknown" : "actions.failed"));
    },
  });
  const on = target.kind === "like" ? state.data.liked : state.data.favorited;
  const unavailable = account.status !== "active";
  const Icon = target.kind === "like" ? Heart : Star;
  const label = t(unavailable ? "actions.login" : `actions.${on ? "remove" : "add"}.${target.kind}`);
  const perform = () => {
    if (queryClient.isMutating({ mutationKey: ["work-action", account.id, target.id, target.kind] })) return;
    mutation.mutate(!on);
  };
  if (compact) return <ControlButton label={label} side="left" className="size-10 bg-viewer-control backdrop-blur-md" aria-pressed={on}
    aria-busy={mutation.isPending} disabled={unavailable} onClick={perform}>
    <Icon className={on ? `fill-current ${activeColor(target.kind)}` : ""} />
  </ControlButton>;
  return <Tooltip><TooltipTrigger asChild><Button variant="ghost" className="h-auto w-full justify-start gap-2 rounded-md px-2 py-2 transition-[scale] duration-(--duration-fast) active:scale-95 motion-reduce:active:scale-100" aria-label={`${account.nickname} · ${label}`} disabled={unavailable} aria-busy={mutation.isPending} onClick={perform}>
    <Avatar className="size-8"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
    <span className="min-w-0 flex-1 text-right text-xs text-muted-foreground">{label}</span>
    <Icon className={on ? `fill-current ${activeColor(target.kind)}` : ""} />
  </Button></TooltipTrigger><TooltipContent side="left" className="z-[70]">{account.nickname} · {label}</TooltipContent></Tooltip>;
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
      return <Tooltip key={account.id}><TooltipTrigger asChild><Button variant="ghost" className="h-auto w-full justify-start gap-2 rounded-md px-2 py-2 transition-[scale] duration-(--duration-fast) active:scale-95 motion-reduce:active:scale-100" aria-label={t(`actions.${on ? "remove" : "add"}.${target.kind}`) + " · " + account.nickname}
        onClick={() => setPreviewRelation(target, account.id, !on)}>
        <Avatar className="size-8"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
        <span className="min-w-0 flex-1 text-right text-xs text-muted-foreground">{t(`actions.${on ? "remove" : "add"}.${target.kind}`)}</span>
        <motion.span key={String(on)} initial={{ scale: 0.8 }} animate={{ scale: 1 }} transition={fastTransition} className="w-4 text-primary">{on && <Check className="size-4" />}</motion.span>
      </Button></TooltipTrigger><TooltipContent side="left" className="z-[70]">{account.nickname} · {t(`actions.${on ? "remove" : "add"}.${target.kind}`)}</TooltipContent></Tooltip>;
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
  if (single) return <ControlButton side="left" className="size-10 bg-viewer-control backdrop-blur-md" label={t(`actions.${on ? "remove" : "add"}.${target.kind}`)} aria-pressed={on}
    onClick={() => setPreviewRelation(target, single.id, !on)}><Icon className={on ? `fill-current ${activeColor(target.kind)}` : ""} /></ControlButton>;
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild>
      <ControlButton side="left" className="size-10 bg-viewer-control backdrop-blur-md" disabled={!accounts.length} label={t(`actions.label.${target.kind}`)}>
        <motion.span key={String(on)} initial={{ scale: 0.8 }} animate={{ scale: 1 }} transition={fastTransition}><Icon className={on ? `fill-current ${activeColor(target.kind)}` : ""} /></motion.span>
      </ControlButton>
    </PopoverTrigger>
    <PopoverContent onOpenAutoFocus={(event) => event.preventDefault()} side="top" align="end" className="dark z-[60] w-44 max-w-[calc(100vw-2rem)] p-2">
      <AccountChoices target={target} />
    </PopoverContent>
  </Popover>;
}

export function ViewerActions({ entry }: { entry: LibraryEntry }) {
  const { t } = useTranslation("library");
  return <div className="flex flex-col gap-2" data-preview-actions aria-label={fixtureName ? t("actions.preview") : undefined}>
    <ActionButton target={{ ...entry, id: entry.id, kind: "like" }} />
    <ActionButton target={{ ...entry, id: entry.id, kind: "favorite" }} />
  </div>;
}

function MenuActionLabel({ entry, kind }: { entry: LibraryEntry; kind: ActionKind }) {
  const { t } = useTranslation("library");
  const { scope, kind: listKind } = useLibraryFilters();
  const preview = usePreviewOwners({ id: entry.id, kind });
  const { data: sources } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources() });
  const candidates = sources?.flatMap((group) => group.accounts).filter((a) => !scope.accountId || a.id === scope.accountId) ?? [];
  const single = candidates.length === 1 ? candidates[0].id : undefined;
  const { data: state } = useQuery<ActionState>({ queryKey: ["action-state", single, entry.id], enabled: false });
  const cached = kind === "like" ? state?.liked : kind === "favorite" ? state?.favorited : undefined;
  const local = !!single && (kind === "like" ? entry.likedAccountIds?.includes(single) : entry.favoritedAccountIds?.includes(single)) || (kind === listKind && !!single && !!entry.accountIds?.includes(single));
  const on = fixtureName ? !!single && preview.includes(single) : cached ?? local;
  const Icon = kind === "like" ? Heart : kind === "favorite" ? Star : Music2;
  return <><Icon className={on ? `fill-current ${activeColor(kind)}` : ""} />{t(`actions.${on ? "remove" : "label"}.${kind}`)}</>;
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
          <DropdownMenuContent>{kinds.map((kind) => <DropdownMenuItem key={kind} onSelect={() => setAction(kind)}><MenuActionLabel entry={entry} kind={kind} /></DropdownMenuItem>)}</DropdownMenuContent>
        </DropdownMenu>
      </div>}
    </div></ContextMenuTrigger>
    <ContextMenuContent>{kinds.map((kind) => <ContextMenuItem key={kind} onSelect={() => setAction(kind)}><MenuActionLabel entry={entry} kind={kind} /></ContextMenuItem>)}</ContextMenuContent>
    <Dialog open={action !== null} onOpenChange={(open) => { if (!open) setAction(null); }}>
      <DialogContent className="sm:max-w-sm">
        <DialogTitle>{t("actions.choose")}</DialogTitle><DialogDescription>{entry.title}{fixtureName ? ` · ${t("actions.preview")}` : ""}</DialogDescription>
        {action && <AccountChoices target={{ ...entry, id: entry.id, kind: action }} />}
      </DialogContent>
    </Dialog>
  </ContextMenu>;
}
