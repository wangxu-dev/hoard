import { useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Heart, Star, Music2, MoreHorizontal, Play } from "lucide-react";
import { motion } from "motion/react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ControlButton } from "@/components/viewer/controls";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem, ContextMenuSub, ContextMenuSubTrigger, ContextMenuSubContent } from "@/components/ui/context-menu";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSub, DropdownMenuSubTrigger, DropdownMenuSubContent } from "@/components/ui/dropdown-menu";
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
  const initial = { liked: target.likedAccountIds?.includes(account.id) ?? false, favorited: target.favoritedAccountIds?.includes(account.id) ?? false, watchLater: false, folders: [] };
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
  return <Tooltip delayDuration={0}><TooltipTrigger asChild><Button variant="ghost" className="relative size-10 shrink-0 rounded-full p-0 transition-[scale] duration-(--duration-fast) active:scale-95 motion-reduce:active:scale-100" aria-label={`${account.nickname} · ${label}`} disabled={unavailable} aria-busy={mutation.isPending} onClick={perform}>
    <Avatar className="size-10"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
    <span className="absolute -right-1 -bottom-1 rounded-full bg-popover p-0.5"><Icon className={`size-3 ${on ? `fill-current ${activeColor(target.kind)}` : "text-muted-foreground"}`} /></span>
  </Button></TooltipTrigger><TooltipContent side="top" className="z-[70]">{account.nickname} · {label}</TooltipContent></Tooltip>;
}

function RealAccountChoices({ target }: { target: ActionTarget }) {
  const { t } = useTranslation("library");
  const { scope } = useLibraryFilters();
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources() });
  if (target.kind === "music") return <p className="text-sm text-muted-foreground">{t("actions.musicUnavailable")}</p>;
  return <div className="flex flex-wrap items-center gap-3" data-account-choices>{data?.filter((group) => group.id === "douyin").flatMap((group) => group.accounts).filter((account) => !scope.accountId || account.id === scope.accountId).map((account) => <RealAccountChoice key={account.id} target={target} account={account} />)}</div>;
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
  return <div className="flex flex-wrap items-center gap-3" data-account-choices>
    {accounts.map((account) => {
      const on = owners.includes(account.id);
      const Icon = target.kind === "like" ? Heart : target.kind === "favorite" ? Star : Music2;
      return <Tooltip key={account.id} delayDuration={0}><TooltipTrigger asChild><Button variant="ghost" className="relative size-10 shrink-0 rounded-full p-0 transition-[scale] duration-(--duration-fast) active:scale-95 motion-reduce:active:scale-100" aria-label={t(`actions.${on ? "remove" : "add"}.${target.kind}`) + " · " + account.nickname}
        onClick={() => setPreviewRelation(target, account.id, !on)}>
        <Avatar className="size-10"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
        <motion.span key={String(on)} initial={{ scale: 0.8 }} animate={{ scale: 1 }} transition={fastTransition} className="absolute -right-1 -bottom-1 rounded-full bg-popover p-0.5"><Icon className={`size-3 ${on ? `fill-current ${activeColor(target.kind)}` : "text-muted-foreground"}`} /></motion.span>
      </Button></TooltipTrigger><TooltipContent side="top" className="z-[70]">{account.nickname} · {t(`actions.${on ? "remove" : "add"}.${target.kind}`)}</TooltipContent></Tooltip>;
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
    <PopoverContent onOpenAutoFocus={(event) => event.preventDefault()} side="left" align="center" sideOffset={12} className="dark z-[60] w-auto max-w-[calc(100vw-6rem)] border-0 bg-transparent p-1 shadow-none">
      <motion.div initial={{ opacity: 0, x: 8 }} animate={{ opacity: 1, x: 0 }} transition={fastTransition}><AccountChoices target={target} /></motion.div>
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

export function EntryActionMenu({ entry, children, onPlay }: { entry: LibraryEntry; children: ReactNode; onPlay: () => void }) {
  const { t } = useTranslation("library");
  const kinds: ActionKind[] = entry.kind === "music" ? ["music"] : ["like", "favorite"];
  return <ContextMenu>
    <ContextMenuTrigger asChild><div className="group/card-menu relative min-w-0">{children}
      {entry.kind === "music" && <div className="absolute right-1.5 top-1.5 opacity-0 transition-opacity duration-base group-hover/card-menu:opacity-100 group-focus-within/card-menu:opacity-100 motion-reduce:transition-none" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
        <DropdownMenu><DropdownMenuTrigger asChild><Button variant="secondary" size="icon" className="size-6" aria-label={t("actions.more")}><MoreHorizontal /></Button></DropdownMenuTrigger>
          <DropdownMenuContent>{entry.kind === "music" && <DropdownMenuItem onSelect={onPlay}><Play />{t("viewer.play")}</DropdownMenuItem>}{kinds.map((kind) => <DropdownMenuSub key={kind}><DropdownMenuSubTrigger><MenuActionLabel entry={entry} kind={kind} /></DropdownMenuSubTrigger><DropdownMenuSubContent className="p-3"><AccountChoices target={{ ...entry, kind }} /></DropdownMenuSubContent></DropdownMenuSub>)}</DropdownMenuContent>
        </DropdownMenu>
      </div>}
    </div></ContextMenuTrigger>
    <ContextMenuContent>{entry.kind === "music" && <ContextMenuItem onSelect={onPlay}><Play />{t("viewer.play")}</ContextMenuItem>}{kinds.map((kind) => <ContextMenuSub key={kind}><ContextMenuSubTrigger className="gap-2"><MenuActionLabel entry={entry} kind={kind} /></ContextMenuSubTrigger><ContextMenuSubContent className="p-3"><AccountChoices target={{ ...entry, kind }} /></ContextMenuSubContent></ContextMenuSub>)}</ContextMenuContent>

  </ContextMenu>;
}
