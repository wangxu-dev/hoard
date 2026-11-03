import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { motion } from "motion/react";
import { Search, SlidersHorizontal, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DropdownMenu, DropdownMenuContent, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { setLibraryFilters, useLibraryFilters, useLibraryOrdering } from "@/lib/library-filters";
import { useQuery } from "@tanstack/react-query";
import { foldersQuery, queryKeys } from "@/lib/query";
import { pageTransition } from "@/lib/motion";
import type { SortKey, WorkType } from "@/lib/library";

export function LibrarySearch({ compact, onExpandedChange }: { compact: boolean; onExpandedChange: (expanded: boolean) => void }) {
  const { t } = useTranslation("library");
  const { queryInput } = useLibraryFilters();
  const [expanded, setExpanded] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const open = !compact || expanded || queryInput.length > 0;
  useEffect(() => { onExpandedChange(expanded || queryInput.length > 0); }, [expanded, queryInput, onExpandedChange]);
  useEffect(() => {
    function shortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "f") {
        event.preventDefault(); setExpanded(true); requestAnimationFrame(() => input.current?.focus());
      }
    }
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, []);
  return <motion.div data-control="search" animate={{ width: open ? 220 : 32 }} transition={pageTransition} className="relative h-8 min-w-0 shrink-0">
    {open ? <><Search aria-hidden="true" className="pointer-events-none absolute top-2 left-2.5 size-3.5 text-muted-foreground" />
      <Input ref={input} value={queryInput} aria-label={t("toolbar.search.label")} placeholder={t("toolbar.search.placeholder")} className="h-8 w-full min-w-0 pr-8 pl-8 text-[13px]" onChange={(e) => setLibraryFilters({ queryInput: e.target.value })}
        onBlur={() => { if (!queryInput) setExpanded(false); }}
        onKeyDown={(e) => { if (e.key === "Escape") { setLibraryFilters({ queryInput: "", query: "" }); setExpanded(false); e.currentTarget.blur(); } }} />
      {queryInput && <Button variant="ghost" size="icon" className="absolute top-0 right-0 size-8" aria-label={t("toolbar.search.clear")} onMouseDown={(e) => e.preventDefault()} onClick={() => { setLibraryFilters({ queryInput: "", query: "" }); input.current?.focus(); }}><X className="size-3.5" /></Button>}
    </> : <Button variant="outline" size="icon" className="size-8" aria-label={t("toolbar.search.label")} onClick={() => { setExpanded(true); requestAnimationFrame(() => input.current?.focus()); }}><Search className="size-3.5" /></Button>}
  </motion.div>;
}
function SortHint({ disabled, reason, children }: { disabled: boolean; reason: string; children: ReactNode }) {
  if (!disabled) return children;
  return <Tooltip><TooltipTrigger asChild><div tabIndex={0} aria-label={reason}>{children}</div></TooltipTrigger><TooltipContent>{reason}</TooltipContent></Tooltip>;
}
export function LibrarySort() {
  const { t } = useTranslation("library");
  const { sort, options } = useLibraryOrdering();
  return <Select value={sort} onValueChange={(v) => setLibraryFilters({ sort: v as SortKey })}>
    <SelectTrigger size="sm" data-control="sort" aria-label={t("toolbar.sort.label")} className="h-8 w-28 min-w-0 shrink-0 text-[13px]"><SelectValue /></SelectTrigger>
    <SelectContent position="popper">{options.map((option) => <SortHint key={option.value} disabled={option.disabled} reason={t(option.reason)}><SelectItem value={option.value} disabled={option.disabled} className="h-8 text-[13px]">{t(option.label)}</SelectItem></SortHint>)}</SelectContent>
  </Select>;
}
export function LibraryFolders() {
  const { t } = useTranslation("library");
  const { scope } = useLibraryFilters();
  const folders = useQuery(foldersQuery(scope.accountId!));
  return <Select value={scope.folderId ?? "all"} onValueChange={(value) => setLibraryFilters({ scope: { ...scope, folderId: value === "all" ? undefined : value } })}>
    <SelectTrigger size="sm" data-control="folders" aria-label={t("folders.label")} className="h-8 w-32 min-w-0 shrink-0 text-[13px]"><SelectValue placeholder={t("folders.all")} /></SelectTrigger>
    <SelectContent position="popper" className="max-h-80">
      <SelectItem value="all">{t("folders.all")}</SelectItem>
      {folders.data?.map((folder) => <SelectItem key={folder.id} value={folder.id}><span className="max-w-48 truncate">{folder.title}</span></SelectItem>)}
      {folders.isError && <Button variant="ghost" size="sm" onClick={() => void folders.refetch()}>{t("error.retry")}</Button>}
    </SelectContent>
  </Select>;
}

export function LibraryFilterMenu() {
  const { t } = useTranslation("library");
  const { kind, type } = useLibraryFilters();
  const { sort, options } = useLibraryOrdering();
  return <DropdownMenu><DropdownMenuTrigger asChild><Button data-control="filters" variant="outline" size="sm" className="h-8 shrink-0 gap-1.5 px-3 text-[13px]"><SlidersHorizontal className="size-3.5" />{t("toolbar.filters")}</Button></DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      {kind !== "music" && <><DropdownMenuLabel>{t("toolbar.type.label")}</DropdownMenuLabel><DropdownMenuRadioGroup value={type} onValueChange={(v) => setLibraryFilters({ type: v as WorkType })}>{(["all", "video", "gallery"] as const).map((v) => <DropdownMenuRadioItem key={v} value={v}>{t(`toolbar.type.${v}`)}</DropdownMenuRadioItem>)}</DropdownMenuRadioGroup><DropdownMenuSeparator /></>}
      <DropdownMenuLabel>{t("toolbar.sort.label")}</DropdownMenuLabel>
      <DropdownMenuRadioGroup value={sort} onValueChange={(v) => setLibraryFilters({ sort: v as SortKey })}>{options.map((option) => <SortHint key={option.value} disabled={option.disabled} reason={t(option.reason)}><DropdownMenuRadioItem value={option.value} disabled={option.disabled}>{t(option.label)}</DropdownMenuRadioItem></SortHint>)}</DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>;
}
