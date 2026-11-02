import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Updates, type UpdateDownload, type UpdateInfo } from "./api";

export const VERSION = import.meta.env.HOARD_VERSION || "dev";
export const RELEASE_NOTES = import.meta.env.HOARD_RELEASE_NOTES || "";
export const REPO_URL = "https://github.com/wangxu-dev/hoard";
const key = ["update"] as const;

// ?update=1 shows an available release in the browser, where there is no Go side.
const demo: UpdateInfo | undefined = import.meta.env.DEV && new URLSearchParams(window.location.search).has("update")
  ? { status: "available", current: VERSION, latest: "v0.2.0", name: "v0.2.0", notes: "- Faster timeline\n- Fixes for live photos", url: `${REPO_URL}/releases/latest`, checkedAt: Date.now(), error: "" }
  : undefined;

/** The daily check runs once at launch; the backend answers from its record when one was made today. */
export function useUpdate() {
  const client = useQueryClient();
  const { data } = useQuery({ queryKey: key, queryFn: () => demo ?? Updates.check(VERSION, false), staleTime: Infinity });
  const check = async () => {
    const info = demo ?? await Updates.check(VERSION, true);
    client.setQueryData(key, info);
    return info;
  };
  const transferKey = ["update-download"] as const;
  const { data: transfer } = useQuery<UpdateDownload>({
    queryKey: transferKey,
    queryFn: () => demo ? ({ status: new URLSearchParams(location.search).get("transfer") || "idle", enabled: true, downloaded: 45, total: 100, error: "", version: demo.latest }) : Updates.downloadStatus(),
    refetchInterval: (query) => query.state.data?.status === "downloading" ? 500 : false,
  });
  const download = async () => {
    if (!data) return;
    if (demo) {
      client.setQueryData(transferKey, { ...transfer, status: "downloading", downloaded: 45, total: 100 });
      return;
    }
    await Updates.download(data.latest);
    await client.invalidateQueries({ queryKey: transferKey });
  };
  const restart = async () => { if (!demo) await Updates.restart(); };
  return { info: data, available: data?.status === "available", check, transfer, download, restart };
}
