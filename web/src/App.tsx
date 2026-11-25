import { Component, lazy, Suspense, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Accounts, App as BackendApp, onAccountsChanged } from "@/lib/api";
import { setLibraryFilters } from "@/lib/library-filters";
import { AnimatePresence, motion, useIsPresent, useReducedMotion } from "motion/react";
import { Toaster } from "@/components/ui/sonner";
import { AppSidebar, type Page } from "@/components/app-sidebar";
import { LibraryPage } from "@/pages/library";
import { SettingsPage } from "@/pages/settings";
import { startLibraryEvents } from "@/lib/library-events";
import { useMusic } from "@/lib/music";
import { MusicBar } from "@/components/viewer/music-bar";
import { cn } from "@/lib/utils";

import { useWindowAppearance } from "@/lib/window-appearance";

const loadExplore = () => import("@/pages/constellation-dev");
const ExplorePage = lazy(loadExplore);

class ExploreBoundary extends Component<{ children: React.ReactNode; message: string; reloadLabel: string }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    if (!this.state.failed) return this.props.children;
    return <div role="alert" className="flex h-full w-full flex-col items-center justify-center gap-4 bg-background text-foreground">
      <p>{this.props.message}</p>
      <button type="button" className="rounded-lg border border-border px-4 py-2 hover:bg-accent" onClick={() => window.location.reload()}>{this.props.reloadLabel}</button>
    </div>;
  }
}

// Keep both complete page surfaces alive during the crossfade. The sidebar's
// different explore/library trees must not unmount the outgoing galaxy early.
function PageSurface({ children, explore }: { children: React.ReactNode; explore: boolean }) {
  const present = useIsPresent();
  const reduced = useReducedMotion();
  return <motion.div data-page-surface="" className="absolute inset-0 flex min-h-0 min-w-0" inert={!present}
    initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
    transition={{ duration: reduced ? 0 : (explore ? .38 : .28), ease: "easeInOut" }}
    style={{ pointerEvents: present ? "auto" : "none" }}>{children}</motion.div>;
}

export function App() {
  const { t } = useTranslation("library");
  const { t: tExplore } = useTranslation("explore");
  const [page, setPage] = useState<Page>("library");
  useWindowAppearance(page);
  const [scrolled, setScrolled] = useState(false);
  const music = useMusic();
  const addingAccount = useRef(false);
  const explorePreloaded = useRef(false);
  const exploreAbort = useRef<AbortController | null>(null);
  const preloadExplore = () => {
    if (explorePreloaded.current) return;
    explorePreloaded.current = true;
    const abort = exploreAbort.current ?? new AbortController();
    exploreAbort.current = abort;
    void loadExplore().then(module => {
      if (!abort.signal.aborted) return module.preloadExploration(abort.signal);
    }).catch(() => {});
  };
  useEffect(startLibraryEvents, []);
  useEffect(() => {
    const abort = new AbortController();
    exploreAbort.current = abort;
    const win = window as Window & {
      requestIdleCallback?: (cb: () => void, opts?: { timeout: number }) => number;
      cancelIdleCallback?: (id: number) => void;
    };
    if (typeof win.requestIdleCallback === "function") {
      const idleId = win.requestIdleCallback(preloadExplore, { timeout: 5000 });
      return () => { win.cancelIdleCallback?.(idleId); abort.abort(); };
    }
    const timer = window.setTimeout(preloadExplore, 4000);
    return () => { clearTimeout(timer); abort.abort(); };
  }, []);
  useEffect(() => onAccountsChanged((accounts) => {
    if (!addingAccount.current) return;
    const active = accounts.find((account) => account.active && account.status === "active");
    if (!active) return;
    addingAccount.current = false;
    setLibraryFilters({ scope: { source: active.source, accountId: active.id } });
  }), []);
  // Showing the playing sound brings the library back from any page.
  useEffect(() => { if (music.locate) setPage("library"); }, [music.locate]);
  const login = async (accountId?: string, source: "douyin" | "bilibili" = "douyin") => {
    try {
      addingAccount.current = !accountId;
      if (accountId) await Accounts.switch(accountId);
      else await Accounts.add(source);
      await BackendApp.showDouyin();
    } catch {
      addingAccount.current = false;
      toast.error(t("error.login"));
    }
  };
  const content = page === "explore" ? <ExploreBoundary message={tExplore("error.message")} reloadLabel={tExplore("error.reload")}><Suspense fallback={<div role="status" className="flex h-full w-full items-center justify-center text-sm text-muted-foreground" style={{ background: "radial-gradient(ellipse at 52% 48%, var(--explore-bg-glow), var(--explore-bg) 72%)" }}>{tExplore("loading")}</div>}><ExplorePage /></Suspense></ExploreBoundary> : page === "library" ? <LibraryPage scrolled={scrolled} onAddAccount={(source) => { void login(undefined, source); }} onLogin={(accountId) => { void login(accountId); }} /> : <SettingsPage />;

  return (
    <div data-app-shell className="relative flex h-dvh min-w-0 overflow-hidden bg-background">
      <AnimatePresence initial={false} mode="sync">
      <PageSurface key={page} explore={page === "explore"}>
      <AppSidebar page={page} onPageChange={setPage} onExploreIntent={preloadExplore}>
        <main className={cn("relative flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background", page === "explore" && "explore-host")}>
          <div data-testid="main-scroll" onScroll={(event) => setScrolled(event.currentTarget.scrollTop > 0)} className={page === "explore" ? "min-h-0 min-w-0 flex-1 overflow-hidden" : "scrollbar-none min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden"}>
            <div style={page === "explore" ? { height: "100%" } : undefined} className={cn("flex min-h-full min-w-0", music.queue && page !== "explore" && "pb-20")}>
              {content}
            </div>
          </div>
        </main>
      </AppSidebar>
      </PageSurface>
      </AnimatePresence>
          {/* The music player sits above every page, so it keeps playing while pages change. */}
          <div className="pointer-events-none absolute inset-x-12 bottom-4 z-30 flex justify-center">
            <AnimatePresence>{music.queue && <MusicBar key="music" className="pointer-events-auto" />}</AnimatePresence>
          </div>

      <Toaster position="bottom-center" />
    </div>
  );
}
