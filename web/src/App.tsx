import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Accounts, App as BackendApp, onAccountsChanged } from "@/lib/api";
import { setLibraryFilters } from "@/lib/library-filters";
import { AnimatePresence, motion } from "motion/react";
import { Toaster } from "@/components/ui/sonner";
import { AppSidebar, type Page } from "@/components/app-sidebar";
import { LibraryPage } from "@/pages/library";
import { SettingsPage } from "@/pages/settings";
import { pageVariants } from "@/lib/motion";
import { startLibraryEvents } from "@/lib/library-events";
import { useMusic } from "@/lib/music";
import { MusicBar } from "@/components/viewer/music-bar";
import { cn } from "@/lib/utils";

const ExplorePage = lazy(() => import("@/pages/constellation-dev"));

export function App() {
  const { t } = useTranslation("library");
  const [page, setPage] = useState<Page>("library");
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
  const content = page === "explore" ? <Suspense fallback={<div className="p-6 text-sm">正在准备探索…</div>}><ExplorePage /></Suspense> : page === "library" ? <LibraryPage scrolled={scrolled} onAddAccount={(source) => { void login(undefined, source); }} onLogin={(accountId) => { void login(accountId); }} /> : <SettingsPage />;

  return (
    <div className="flex h-dvh min-w-0 overflow-hidden bg-background">
      <AppSidebar page={page} onPageChange={setPage}>
        <main className="relative flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background">
          <div data-testid="main-scroll" onScroll={(event) => setScrolled(event.currentTarget.scrollTop > 0)} className={page === "explore" ? "min-h-0 min-w-0 flex-1 overflow-hidden" : "scrollbar-none min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden"}>
            <AnimatePresence mode="wait" initial={false}>
              <motion.div style={page === "explore" ? { height: "100%" } : undefined} key={page} variants={pageVariants} initial="initial" animate="animate" exit="exit" className={cn("flex min-h-full min-w-0", music.queue && "pb-20")}>
                {content}
              </motion.div>
            </AnimatePresence>
          </div>
          {/* The music player sits above every page, so it keeps playing while pages change. */}
          <div className="pointer-events-none absolute inset-x-12 bottom-4 z-30 flex justify-center">
            <AnimatePresence>{music.queue && <MusicBar key="music" className="pointer-events-auto" />}</AnimatePresence>
          </div>
        </main>
      </AppSidebar>
      <Toaster position="bottom-center" />
    </div>
  );
}
