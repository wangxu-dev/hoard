import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Heart, ThumbsUp, Star, Clock, Music2, MoreHorizontal, Play } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ControlButton } from "@/components/viewer/controls";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem, ContextMenuSub, ContextMenuSubTrigger, ContextMenuSubContent } from "@/components/ui/context-menu";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSub, DropdownMenuSubTrigger, DropdownMenuSubContent } from "@/components/ui/dropdown-menu";
import { FavoriteFolders } from "@/components/favorite-folders";
import { getLibrarySource, type LibraryEntry, type SourceAccount } from "@/lib/library";
import { useLibraryFilters } from "@/lib/library-filters";
import { usePreviewOwners, setPreviewRelation, setPreviewFolders, type ActionTarget, type ActionKind } from "@/lib/action-preview";
import { fixtureName } from "@/lib/demo";
import type { ActionState } from "@/lib/api";
import { queryKeys } from "@/lib/query";
import { actionField, actionKey, useWorkAction } from "@/lib/work-actions";

const activeColor = (kind: ActionKind) => kind === "like" ? "text-(--action-like)" : kind === "favorite" ? "text-(--action-favorite)" : "text-(--action-watch-later)";
const activeIcon = (kind: ActionKind) => `${kind === "watch_later" ? "" : "fill-current "}${activeColor(kind)}`;
const iconFor = (target: ActionTarget) => target.kind === "like" ? target.source === "bilibili" ? ThumbsUp : Heart : target.kind === "favorite" ? Star : target.kind === "watch_later" ? Clock : Music2;
const labelKey = (target: ActionTarget, on: boolean) => `actions.${on ? "remove" : "add"}.${target.kind === "like" && target.source === "bilibili" ? "bilibili_like" : target.kind}`;

function useActionAccounts(target: ActionTarget) {
  const { scope } = useLibraryFilters();
  const { data } = useQuery({ queryKey: queryKeys.sources, queryFn: () => getLibrarySource().sources() });
  return data?.filter((group) => group.id === (target.source ?? "douyin")).flatMap((group) => group.accounts).filter((account) => !scope.accountId || account.id === scope.accountId) ?? [];
}

function AccountControl({ target, account, compact, on, pending, perform }: { target: ActionTarget; account: SourceAccount; compact: boolean; on: boolean; pending: boolean; perform: (on: boolean, folders?: string[]) => void }) {
  const { t } = useTranslation("library");
  const [open, setOpen] = useState(false);
  const unavailable = account.status !== "active";
  const Icon = iconFor(target);
  const label = t(unavailable ? "actions.login" : labelKey(target, on));
  const chooseFolders = target.source === "bilibili" && target.kind === "favorite" && !target.id.startsWith("opus:");
  const button = compact ? <ControlButton label={label} side="left" className="size-10 bg-viewer-control backdrop-blur-md" aria-pressed={on} aria-busy={pending} disabled={unavailable} onClick={chooseFolders ? undefined : () => perform(!on)}>
    <Icon className={on ? activeIcon(target.kind) : ""} />
  </ControlButton> : <Button variant="ghost" className="relative size-10 shrink-0 rounded-full p-0" aria-label={`${account.nickname} · ${label}`} disabled={unavailable} aria-pressed={on} aria-busy={pending} onClick={chooseFolders ? undefined : () => perform(!on)}>
    <Avatar className="size-10"><AvatarImage src={account.avatarUrl} alt="" /><AvatarFallback>{account.nickname.slice(0, 1)}</AvatarFallback></Avatar>
    <span className="absolute -right-1 -bottom-1 rounded-full bg-popover p-0.5"><Icon className={`size-3 ${on ? activeIcon(target.kind) : "text-muted-foreground"}`} /></span>
  </Button>;
  if (!chooseFolders) return compact ? button : <Tooltip><TooltipTrigger asChild>{button}</TooltipTrigger><TooltipContent side="top" className="z-[80]">{account.nickname} · {label}</TooltipContent></Tooltip>;
  return <Popover open={open} onOpenChange={setOpen}><PopoverTrigger asChild>{button}</PopoverTrigger>
    <PopoverContent side={compact ? "left" : "bottom"} className="z-[80] w-auto p-3"><FavoriteFolders target={target} accountId={account.id} pending={pending} onSave={(ids) => { perform(ids.length > 0, ids); setOpen(false); }} /></PopoverContent>
  </Popover>;
}

function RealAccountChoice({ target, account, compact = false }: { target: ActionTarget; account: SourceAccount; compact?: boolean }) {
  const action = useWorkAction(target, account.id);
  return <AccountControl target={target} account={account} compact={compact} {...action} />;
}

function PreviewAccountChoice({ target, account, compact = false }: { target: ActionTarget; account: SourceAccount; compact?: boolean }) {
  const owners = usePreviewOwners(target);
  return <AccountControl target={target} account={account} compact={compact} on={owners.includes(account.id)} pending={false} perform={(on, ids) => { if (ids) setPreviewFolders(target, account.id, ids); else setPreviewRelation(target, account.id, on); }} />;
}

function AccountChoice(props: { target: ActionTarget; account: SourceAccount; compact?: boolean }) {
  return fixtureName ? <PreviewAccountChoice {...props} /> : <RealAccountChoice {...props} />;
}

function AccountChoices({ target }: { target: ActionTarget }) {
  const { t } = useTranslation("library");
  const accounts = useActionAccounts(target);
  if (target.kind === "music") return <p className="text-sm text-muted-foreground">{t("actions.musicUnavailable")}</p>;
  return <div className="flex flex-wrap items-center gap-3" data-account-choices>{accounts.map((account) => <AccountChoice key={account.id} target={target} account={account} />)}</div>;
}

function ActionButton({ target }: { target: ActionTarget }) {
  const { t } = useTranslation("library");
  const accounts = useActionAccounts(target);
  const Icon = iconFor(target);
  if (accounts.length === 1) return <AccountChoice target={target} account={accounts[0]} compact />;
  return <Popover><PopoverTrigger asChild><ControlButton side="left" className="size-10 bg-viewer-control backdrop-blur-md" disabled={!accounts.length} label={t(`actions.label.${target.kind}`)}><Icon /></ControlButton></PopoverTrigger>
    <PopoverContent side="left" align="center" sideOffset={12} className="dark z-[60] w-auto max-w-[calc(100vw-6rem)] border-0 bg-transparent p-1 shadow-none"><AccountChoices target={target} /></PopoverContent>
  </Popover>;
}

const kindsFor = (entry: LibraryEntry): ActionKind[] => entry.kind === "music" ? ["music"] : entry.source === "bilibili" ? entry.kind === "video" ? ["favorite", "watch_later"] : ["favorite"] : ["like", "favorite"];

export function ViewerActions({ entry }: { entry: LibraryEntry }) {
  const { t } = useTranslation("library");
  return <div className="flex flex-col gap-2" data-preview-actions aria-label={fixtureName ? t("actions.preview") : undefined}>{kindsFor(entry).map((kind) => <ActionButton key={kind} target={{ ...entry, kind }} />)}</div>;
}

function MenuActionLabel({ entry, kind }: { entry: LibraryEntry; kind: ActionKind }) {
  const { t } = useTranslation("library");
  const target = { ...entry, kind };
  const accounts = useActionAccounts(target);
  const preview = usePreviewOwners(target);
  const single = accounts.length === 1 ? accounts[0].id : undefined;
  const { data: state } = useQuery<ActionState>({ queryKey: actionKey(single ?? "", entry.id), enabled: false });
  const local = kind === "like" ? entry.likedAccountIds : kind === "watch_later" ? entry.watchLaterAccountIds : entry.favoritedAccountIds;
  const on = !!single && (fixtureName ? preview.includes(single) : state?.[actionField(kind)] ?? local?.includes(single));
  const Icon = iconFor(target);
  return <><Icon className={on ? activeIcon(kind) : ""} />{t(on ? labelKey(target, true) : `actions.label.${kind === "like" && entry.source === "bilibili" ? "bilibili_like" : kind}`)}</>;
}

export function EntryActionMenu({ entry, children, onPlay }: { entry: LibraryEntry; children: ReactNode; onPlay: () => void }) {
  const { t } = useTranslation("library");
  const kinds = kindsFor(entry);
  return <ContextMenu>
    <ContextMenuTrigger asChild><div className="group/card-menu relative min-w-0">{children}
      {entry.kind === "music" && <div className="absolute right-1.5 top-1.5 opacity-0 transition-opacity duration-base group-hover/card-menu:opacity-100 group-focus-within/card-menu:opacity-100" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
        <DropdownMenu><DropdownMenuTrigger asChild><Button variant="secondary" size="icon" className="size-6" aria-label={t("actions.more")}><MoreHorizontal /></Button></DropdownMenuTrigger>
          <DropdownMenuContent><DropdownMenuItem onSelect={onPlay}><Play />{t("viewer.play")}</DropdownMenuItem>{kinds.map((kind) => <DropdownMenuSub key={kind}><DropdownMenuSubTrigger><MenuActionLabel entry={entry} kind={kind} /></DropdownMenuSubTrigger><DropdownMenuSubContent className="p-3"><AccountChoices target={{ ...entry, kind }} /></DropdownMenuSubContent></DropdownMenuSub>)}</DropdownMenuContent>
        </DropdownMenu>
      </div>}
    </div></ContextMenuTrigger>
    <ContextMenuContent>{entry.kind === "music" && <ContextMenuItem onSelect={onPlay}><Play />{t("viewer.play")}</ContextMenuItem>}{kinds.map((kind) => <ContextMenuSub key={kind}><ContextMenuSubTrigger className="gap-2"><MenuActionLabel entry={entry} kind={kind} /></ContextMenuSubTrigger><ContextMenuSubContent className="p-3"><AccountChoices target={{ ...entry, kind }} /></ContextMenuSubContent></ContextMenuSub>)}</ContextMenuContent>
  </ContextMenu>;
}
