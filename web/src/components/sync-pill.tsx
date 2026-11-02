import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Check, Clock, LogIn, RefreshCw, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { queryClient, queryKeys } from "@/lib/query";
import { Spinner } from "@/components/ui/spinner";
import { AnimatedShinyText } from "@/components/ui/animated-shiny-text";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { LibraryNumber } from "@/components/library-number";
import { pageTransition } from "@/lib/motion";
import { fixtureFrozen } from "@/lib/demo";
import { getLibrarySource, type SyncState } from "@/lib/library";
import { formatWhen } from "@/lib/time";

const notified = new Map<string, number>();
const freshFor = 4000;
const busy = (s: SyncState) => s.status === "running" || s.status === "checking" || s.status === "queued" || s.status === "cooling" || s.status === "verifying";
// A list that still has backfill left keeps syncing on its own, so between turns and across restarts it reads as syncing.
const backfilling = (s: SyncState) => s.phase !== "done" && !["paused", "unsupported", "signed_out", "blocked", "verifying", "cooling"].includes(s.status);
const checking = (s: SyncState) => s.phase === "done" && (s.status === "checking" || s.status === "queued" || s.status === "running");
export function useSyncVisible(state?: SyncState) {
  return !!state;
}
export function SyncPill({ state, states, visible, compact = false, onLogin }: { state?: SyncState; states: SyncState[]; visible: boolean; compact?: boolean; onLogin: (accountId: string) => void }) {
  const { t, i18n } = useTranslation("library");
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));
  const submitting = useRef(false);
  const lastClick = useRef(0);
  const [fresh, setFresh] = useState<Record<string, number>>({});
  const freshRows = states.filter((s) => s.fresh).map((s) => `${s.accountId}:${s.kind}:${s.fresh}`).join(",");
  useEffect(() => {
    const found = states.filter((s) => s.fresh);
    if (!found.length) return;
    setFresh(Object.fromEntries(found.map((s) => [`${s.accountId}:${s.kind}`, s.fresh ?? 0])));
  }, [freshRows]);
  useEffect(() => {
    if (!Object.keys(fresh).length) return;
    const timer = setTimeout(() => setFresh({}), freshFor);
    return () => clearTimeout(timer);
  }, [fresh]);
  useEffect(() => {
    if (state?.status === "cooling") {
      const key = `${state.accountId}:${state.kind}`;
      if (state.retryAt !== undefined && notified.get(key) !== state.retryAt) {
        notified.set(key, state.retryAt); toast(t("sync.toast"), { id: "sync-cooling" });
      }
    }
    if (fixtureFrozen) return;
    const timer = setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => clearInterval(timer);
  }, [state?.status, state?.retryAt, state?.accountId, state?.kind, t]);
  if (!state) return null;
  const order = ["like", "favorite", "music"];
  const rows = states.filter((s) => s.accountId === state.accountId && s.kind !== "cover").sort((a, b) => order.indexOf(a.kind) - order.indexOf(b.kind));
  const anyBusy = rows.some(busy);
  const format = (n: number) => new Intl.NumberFormat(i18n.language).format(n);
  const remaining = Math.max(0, (state.retryAt ?? now) - (fixtureFrozen && state.retryAt ? state.retryAt - 118 : now));
  const time = `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, "0")}`;
  const kindName = (s: SyncState) => s.kind === "cover" ? t("sync.popover.cover") : t(`toolbar.kind.${s.kind}`);
  const progress = (s: SyncState) => `${format(s.items)}${s.remoteTotal === undefined ? "" : ` / ${format(s.remoteTotal)}`}`;
  const lastSync = Math.max(0, ...rows.map((s) => s.lastSyncAt ?? 0));
  const freshCount = (s: SyncState) => fresh[`${s.accountId}:${s.kind}`] ?? 0;
  function listLabel(s: SyncState) {
    return s.status === "signed_out" ? t("sync.loginRequired")
      : s.status === "blocked" ? t("sync.verifyRequired")
      : s.status === "verifying" ? t("sync.verifying")
      : s.status === "cooling" ? t("sync.cooling", { time })
      : s.status === "paused" ? t("sync.paused")
      : s.status === "unsupported" ? t("sync.unsupported")
      : backfilling(s) ? `${t("sync.syncing")} · ${progress(s)}`
      : checking(s) ? t("sync.checking")
      : freshCount(s) ? t("sync.fresh", { n: format(freshCount(s)) })
      : t("sync.synced");
  }
  // The same text as listLabel, with counts drawn by the number ticker.
  function listNode(s: SyncState) {
    const total = s.remoteTotal === undefined ? null : <> / {format(s.remoteTotal)}</>;
    if (backfilling(s)) return <>{t("sync.syncing")} · <LibraryNumber value={s.items} />{total}</>;
    if (s.status === "paused" || s.status === "unsupported" || s.status === "signed_out" || s.status === "blocked" || s.status === "verifying" || s.status === "cooling" || checking(s) || freshCount(s)) return listLabel(s);
    return t("sync.synced");
  }
  const label = state.status === "cooling" ? `${kindName(state)} · ${listLabel(state)}` : listLabel(state);
  const live = backfilling(state);
  const action = state.status === "signed_out" ? t("sync.login")
    : state.status === "blocked" || state.status === "verifying" ? t("sync.verify")
    : anyBusy ? listLabel(state) : t("sync.clickToSync");
  const timeLabel = lastSync ? (now - lastSync < 180 ? t("sync.justNow") : formatWhen(lastSync, i18n.language)) : t("sync.timeUnknown");
  const showTime = label === t("sync.synced");
  const start = async () => {
    if (submitting.current || Date.now() - lastClick.current < 1000) return;
    lastClick.current = Date.now();
    if (state.status === "signed_out") { onLogin(state.accountId); return; }
    if (state.status === "cooling") return;
    submitting.current = true;
    const verifying = state.status === "blocked" || state.status === "verifying";
    try {
      if (verifying) await getLibrarySource().verify();
      else await getLibrarySource().startSync(state.accountId);
      await queryClient.invalidateQueries({ queryKey: queryKeys.syncStates });
    } catch {
      toast.error(t(verifying ? "sync.verifyFailed" : "sync.startFailed"));
    } finally {
      submitting.current = false;
    }
  };
  const tone = state.status === "signed_out" || state.status === "blocked" ? "border-destructive/40 bg-destructive/10 text-destructive" : state.status === "cooling" ? "text-muted-foreground" : "";
  const icon = state.status === "signed_out" ? <LogIn className="size-3 shrink-0" />
    : state.status === "blocked" ? <ShieldAlert className="size-3 shrink-0" />
    : state.status === "cooling" ? <Clock className="size-3 shrink-0" />
    : backfilling(state) || checking(state) ? <Spinner className="size-3 shrink-0" />
    : state.status === "unsupported" ? <RefreshCw className="size-3 shrink-0" /> : <Check className="size-3 shrink-0" />;
  return <AnimatePresence initial={false}>{visible && <motion.div layout data-control="pill" data-compact={compact} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0, width: 0 }} transition={pageTransition} className="min-w-6 overflow-hidden">
    <Tooltip><TooltipTrigger asChild>
      <Badge asChild variant="outline" className={`h-6 cursor-pointer rounded-full px-2.5 py-0 text-xs font-normal tabular-nums ${tone}`}>
        <button aria-label={`${t("sync.label")}: ${label}`} aria-busy={anyBusy} type="button" onClick={() => void start()}>
          {icon}
          <span data-pill-label className="shrink-0 whitespace-nowrap">{live ? <><AnimatedShinyText className="sync-shine mx-0 text-foreground dark:text-foreground">{`${t("sync.syncing")} ·`}</AnimatedShinyText>{" "}<LibraryNumber value={state.items} />{state.remoteTotal === undefined ? null : <> / {format(state.remoteTotal)}</>}</> : state.status === "cooling" ? label : listNode(state)}</span>
          {showTime && <span className="whitespace-nowrap text-muted-foreground">· {timeLabel}</span>}
        </button>
      </Badge>
    </TooltipTrigger><TooltipContent side="bottom">
      {action}
    </TooltipContent></Tooltip>
  </motion.div>}</AnimatePresence>;
}
