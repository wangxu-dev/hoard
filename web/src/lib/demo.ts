export type FixtureName = "indexing" | "cooling" | "synced" | "unknown-total" | "login-required" | "idle-real-layout";
const params = new URLSearchParams(window.location.search);
const requested = import.meta.env.DEV ? params.get("fixture") ?? import.meta.env.HOARD_FIXTURE : undefined;
export const fixtureName: FixtureName | undefined = requested === undefined || requested === null ? undefined
  : (["indexing", "cooling", "synced", "unknown-total", "login-required", "idle-real-layout"].includes(requested) ? requested as FixtureName : "indexing");
export const fixtureFrozen = import.meta.env.DEV && params.get("freeze") === "1";
export function initializeDemoTheme() {
  if (!import.meta.env.DEV) return;
  const theme = params.get("theme") ?? import.meta.env.HOARD_THEME;
  if (theme === "light" || theme === "dark") localStorage.setItem("hoard-theme", theme);
}
