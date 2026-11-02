import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { LibraryNumber } from "@/components/library-number";
import { fastTransition, pageTransition } from "@/lib/motion";
import { setLibraryFilters, useLibraryFilters } from "@/lib/library-filters";
import type { LibraryCounts, ListKind, WorkType } from "@/lib/library";
export function ListToggle({ kinds, counts }: { kinds: ListKind[]; counts: LibraryCounts }) {
  const { t } = useTranslation("library");
  const { kind } = useLibraryFilters();
  const content = (k: ListKind) => <><span>{t(`toolbar.kind.${k}`)}</span><span className="text-xs font-normal text-muted-foreground"><LibraryNumber value={counts[k]} /></span></>;
  if (kinds.length === 1) return <span data-control="lists" className="flex h-8 min-w-0 items-center gap-1.5 text-[13px] font-medium">{content(kinds[0])}</span>;
  // The active background slides between tabs through a shared layout id.
  return <ToggleGroup data-control="lists" type="single" variant="outline" size="sm" value={kind} onValueChange={(v) => { if (v) setLibraryFilters({ kind: v as ListKind }); }} aria-label={t("toolbar.kind.label")} className="min-w-0 shrink-0">
    {kinds.map((k) => <ToggleGroupItem key={k} value={k} className="relative h-8 gap-1.5 px-3 text-[13px] font-medium data-[state=on]:bg-transparent data-[state=on]:text-foreground">
      {k === kind && <motion.span layoutId="list-toggle-active" aria-hidden="true" className="absolute inset-0 rounded-[inherit] bg-accent" transition={pageTransition} />}
      <span className="relative flex items-center gap-1.5">{content(k)}</span>
    </ToggleGroupItem>)}
  </ToggleGroup>;
}
export function TypeToggle() {
  const { t } = useTranslation("library");
  const { kind, type } = useLibraryFilters();
  return <motion.div hidden={kind === "music"} aria-hidden={kind === "music"} data-control="types" initial={false} animate={{ opacity: kind === "music" ? 0 : 1 }} transition={fastTransition} className="min-w-0 overflow-hidden">
    <ToggleGroup type="single" size="sm" value={type} onValueChange={(v) => { if (v) setLibraryFilters({ type: v as WorkType }); }} aria-label={t("toolbar.type.label")}>
      {(["all", "video", "gallery"] as const).map((v) => <ToggleGroupItem key={v} value={v} className="h-8 px-3 text-[13px] data-[state=on]:bg-accent data-[state=on]:text-foreground">{t(`toolbar.type.${v}`)}</ToggleGroupItem>)}
    </ToggleGroup>
  </motion.div>;
}
