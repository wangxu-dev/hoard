import "./styles.css";
import "../../web/src/i18n";
import { useState } from "react";
import { createRoot } from "react-dom/client";
import { MotionConfig } from "motion/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { demoEntries } from "./backend";
import { queryKeys } from "../../web/src/lib/query";
import { queryClient } from "../../web/src/lib/query";
import { ThemeProvider } from "../../web/src/providers/theme-provider";
import { AppSidebar } from "../../web/src/components/app-sidebar";
import { LibraryPage } from "../../web/src/pages/library";
import { Collection } from "./collection";
import { Toaster } from "../../web/src/components/ui/sonner";

const initialTheme =
  new URLSearchParams(location.search).get("theme") === "dark"
    ? "dark"
    : "light";
localStorage.setItem("hoard-theme", initialTheme);
localStorage.setItem("hoard-sidebar", "true");
window.addEventListener("message", (event) => {
  if (
    event.origin !== location.origin ||
    event.source !== parent ||
    event.data?.type !== "hoard-preview-theme"
  )
    return;
  const next = event.data.theme;
  if (next !== "light" && next !== "dark") return;
  localStorage.setItem("hoard-theme", next);
  window.dispatchEvent(
    new StorageEvent("storage", { key: "hoard-theme", newValue: next }),
  );
});

function Preview() {
  const [scrolled, setScrolled] = useState(false);
  const hint = () => {};
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider
        attribute="class"
        defaultTheme={initialTheme}
        enableSystem={false}
        storageKey="hoard-theme"
      >
        <MotionConfig reducedMotion="user">
          {new URLSearchParams(location.search).get("view") === "collection" ? (
            <Collection />
          ) : (
            <div
              className="flex h-dvh min-w-0 overflow-hidden bg-background"
              onClickCapture={(event) => {
                if (
                  (event.target as HTMLElement).closest(
                    '[data-testid="library-card"]',
                  )
                ) {
                  event.preventDefault();
                  event.stopPropagation();
                }
              }}
              onKeyDownCapture={(event) => {
                if (
                  (event.key === "Enter" || event.key === " ") &&
                  (event.target as HTMLElement).closest(
                    '[data-testid="library-card"]',
                  )
                ) {
                  event.preventDefault();
                  event.stopPropagation();
                }
              }}
            >
              <AppSidebar page="library" onPageChange={hint}>
                <main className="relative flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background">
                  <div
                    data-testid="main-scroll"
                    onScroll={(event) =>
                      setScrolled(event.currentTarget.scrollTop > 0)
                    }
                    className="scrollbar-none min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden"
                  >
                    <div className="flex min-h-full min-w-0">
                      <LibraryPage
                        scrolled={scrolled}
                        onAddAccount={hint}
                        onLogin={hint}
                      />
                    </div>
                  </div>
                </main>
              </AppSidebar>
              <Toaster position="bottom-center" />
            </div>
          )}
        </MotionConfig>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

queryClient.setQueryData(
  queryKeys.grid(JSON.stringify([null, null, "like", "all", "", "published"])),
  { items: demoEntries, total: demoEntries.length, shift: 0 },
);
createRoot(document.getElementById("root")!).render(<Preview />);
