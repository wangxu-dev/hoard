import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Folder } from "lucide-react";
import { Actions, type ActionFolder } from "@/lib/api";
import { previewFolders, type ActionTarget } from "@/lib/action-preview";
import { fixtureName } from "@/lib/demo";
import { getLibrarySource } from "@/lib/library";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";

export function FavoriteFolders({ target, accountId, pending, onSave }: { target: ActionTarget; accountId: string; pending: boolean; onSave: (ids: string[]) => void }) {
  const { t } = useTranslation("library");
  const folders = useQuery({ queryKey: ["action-folders", accountId, target.id], queryFn: async (): Promise<ActionFolder[]> => fixtureName
    ? (await getLibrarySource().folders(accountId)).map((folder) => ({ ...folder, selected: previewFolders(target, accountId).includes(folder.id) })) : Actions.folders(accountId, target.id), staleTime: 0 });
  const [selection, setSelection] = useState<string[] | null>(null);
  const selected = selection ?? folders.data?.filter((folder) => folder.selected).map((folder) => folder.id) ?? [];
  const toggle = (id: string) => setSelection(selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id]);
  return <div className="w-56 space-y-2" aria-label={t("folders.choose")}>
    <p className="px-2 text-sm font-medium">{t("folders.choose")}</p>
    {folders.isPending ? <p role="status" className="px-2 text-sm text-muted-foreground">{t("folders.loading")}</p>
      : folders.isError ? <Button variant="ghost" onClick={() => void folders.refetch()}>{t("error.retry")}</Button>
      : !folders.data?.length ? <p className="px-2 text-sm text-muted-foreground">{t("folders.empty")}</p>
      : <><div className="max-h-64 overflow-y-auto"><div className="space-y-1">{folders.data.map((folder) => <label key={folder.id} className="flex min-h-9 cursor-pointer items-center gap-2 rounded-md px-2 hover:bg-accent">
        <Checkbox checked={selected.includes(folder.id)} disabled={pending} onCheckedChange={() => toggle(folder.id)} />
        <Folder className="size-4 shrink-0" /><span className="min-w-0 flex-1 truncate text-sm">{folder.title}</span>
      </label>)}</div></div><Button className="w-full" disabled={pending || selection === null} onClick={() => onSave(selected)}>{t("folders.save")}</Button></>}
  </div>;
}
