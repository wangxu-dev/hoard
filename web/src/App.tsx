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

const ExplorePage = lazy(() => import("@/pages/constellation-dev"));

class ExploreBoundary extends Component<{ children: React.ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    if (!this.state.failed) return this.props.children;
    return <div role="alert" className="flex h-full w-full flex-col items-center justify-center gap-4" style={{ background: "#0b131d", color: "#dbe9ed" }}>
      <p>暂时无法加载探索，收藏数据不受影响。</p>
      <button className="rounded-lg border border-white/20 px-4 py-2" onClick={() => window.location.reload()}>重新加载界面</button>
    </div>;
  }
}

// Keep both complete page surfaces alive during the crossfade. The sidebar's
// different explore/library trees must not unmount the outgoing galaxy early.
function PageSurface({ children, explore }: { children: React.ReactNode; explore: boolean }) {
  const present = useIsPresent();
  const reduced = useReducedMotion();
  return <motion.div className="absolute inset-0 flex min-h-0 min-w-0" inert={!present}
    initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
    transition={{ duration: reduced ? 0 : (explore ? .38 : .28), ease: "easeInOut" }}
    style={{ pointerEvents: present ? "auto" : "none" }}>{children}</motion.div>;
}

export function App() {
  const { t } = useTranslation("library");
  const [page, setPage] = useState<Page>("library");
  useWindowAppearance(page);
  const [scrolled, setScrolled] = useState(false);
  const music = useMusic();
  const addingAccount = useRef(false);
  useEffect(startLibraryEvents, []);
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
  const content = page === "explore" ? <ExploreBoundary><Suspense fallback={<div role="status" className="flex h-full w-full items-center justify-center text-sm" style={{background:"radial-gradient(ellipse at 52% 48%,#142c39,#0b131d 72%)",color:"#a0b3bf"}}>正在准备探索…</div>}><ExplorePage /></Suspense></ExploreBoundary> : page === "library" ? <LibraryPage scrolled={scrolled} onAddAccount={(source) => { void login(undefined, source); }} onLogin={(accountId) => { void login(accountId); }} /> : <SettingsPage />;

  return (
    <div data-app-shell className="relative flex h-dvh min-w-0 overflow-hidden bg-background">
      <AnimatePresence initial={false} mode="sync">
      <PageSurface key={page} explore={page === "explore"}>
      <AppSidebar page={page} onPageChange={setPage}>
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
