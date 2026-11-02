import "./styles/globals.css";
import "./i18n";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MotionConfig } from "motion/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "./lib/query";
import { App } from "./App";
import { ThemeProvider } from "./providers/theme-provider";
import { initializeDemoTheme } from "./lib/demo";

initializeDemoTheme();

// The default WebKit menu (Reload, Inspect, Look Up…) belongs to web pages;
// fields keep theirs for copy and paste, and development builds keep it all.
if (!import.meta.env.DEV) document.addEventListener("contextmenu", (event) => {
  if (!(event.target instanceof Element && event.target.closest("input, textarea, [contenteditable=true], .select-text"))) event.preventDefault();
});
// Focus rings answer the keyboard: pointer use hides them until a key is pressed.
const modality = (value: "pointer" | "keyboard") => { if (document.documentElement.dataset.modality !== value) document.documentElement.dataset.modality = value; };
document.addEventListener("pointerdown", () => modality("pointer"), true);
document.addEventListener("keydown", (event) => { if (!event.metaKey && !event.ctrlKey && event.key !== "Meta" && event.key !== "Shift" && event.key !== "Alt" && event.key !== "Control") modality("keyboard"); }, true);
// A double or triple click acts like a click: no word or line selection, no focus change.
document.addEventListener("mousedown", (event) => {
  if (event.detail > 1 && !(event.target instanceof Element && event.target.closest("input, textarea, [contenteditable=true], .select-text"))) event.preventDefault();
}, true);
// Dragging an image or link out of the window is a browser habit, not an app one.
document.addEventListener("dragstart", (event) => {
  if (!(event.target instanceof Element && event.target.closest("[draggable=true]"))) event.preventDefault();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem storageKey="hoard-theme">
        <MotionConfig reducedMotion="user">
          <App />
        </MotionConfig>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
