import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Actions, type ActionState } from "./api";
import type { ActionKind, ActionTarget } from "./action-preview";
import { queryClient } from "./query";

export const actionField = (kind: ActionKind) => kind === "like" ? "liked" : kind === "watch_later" ? "watchLater" : "favorited";
export const actionKey = (accountId: string, workId: string) => ["action-state", accountId, workId];

export function useWorkAction(target: ActionTarget, accountId: string) {
  const { t } = useTranslation("library");
  const key = actionKey(accountId, target.id);
  const field = actionField(target.kind);
  const initial: ActionState = {
    liked: target.likedAccountIds?.includes(accountId) ?? false,
    favorited: target.favoritedAccountIds?.includes(accountId) ?? false,
    watchLater: target.watchLaterAccountIds?.includes(accountId) ?? false,
    folders: [],
  };
  const state = useQuery({ queryKey: key, queryFn: () => Actions.state(accountId, target.id), initialData: initial, staleTime: 0 });
  const mutation = useMutation({
    mutationKey: ["work-action", accountId, target.id, target.kind],
    mutationFn: ({ on, folderIds }: { on: boolean; folderIds?: string[] }) => folderIds
      ? Actions.setFolders(accountId, target.id, folderIds) : Actions.set(accountId, target.id, target.kind, on),
    onMutate: async ({ on }: { on: boolean; folderIds?: string[] }) => {
      await queryClient.cancelQueries({ queryKey: key });
      const before = queryClient.getQueryData<ActionState>(key)?.[field] ?? initial[field];
      queryClient.setQueryData<ActionState>(key, (old = initial) => ({ ...old, [field]: on }));
      return { before };
    },
    onSuccess: (result) => {
      queryClient.setQueryData<ActionState>(key, (old = initial) => ({ ...old, [field]: result[field], ...(field === "favorited" ? { folders: result.folders } : {}) }));
      void queryClient.invalidateQueries({ queryKey: ["action-folders", accountId, target.id] });
      void queryClient.invalidateQueries({ queryKey: ["folders", accountId] });
      void queryClient.invalidateQueries({ queryKey: ["counts"] });
    },
    onError: (error: Error, _next, context) => {
      if (context) queryClient.setQueryData<ActionState>(key, (old = initial) => ({ ...old, [field]: context.before }));
      toast.error(t(error.message.includes("ACTION_UNKNOWN") ? "actions.unknown" : "actions.failed"));
    },
  });
  const perform = (on: boolean, folderIds?: string[]) => {
    if (queryClient.isMutating({ mutationKey: ["work-action", accountId, target.id, target.kind] })) return;
    mutation.mutate({ on, folderIds });
  };
  return { on: state.data[field], pending: mutation.isPending, perform };
}
