import { useEffect } from "react";
import { useReducedMotion } from "motion/react";
import { useTheme } from "next-themes";
import { WindowAppearance } from "@mygo";

let pending = Promise.resolve();
let appliedColor: string | null = null;
// Resolve the actually painted CSS color (including OKLCH) into native sRGB.
function nativeColor(css: string, fallback: string): string {
 const ctx = document.createElement("canvas").getContext("2d", {willReadFrequently:true});
 if (!ctx) return fallback;
 ctx.fillStyle=fallback; ctx.fillStyle=css; ctx.fillRect(0,0,1,1);
 const rgba=ctx.getImageData(0,0,1,1).data;
 return "#"+Array.from(rgba.slice(0,3),v=>v.toString(16).padStart(2,"0")).join("");
}
export function useWindowAppearance(page: string) {
  const { resolvedTheme } = useTheme();
  const reduced = useReducedMotion();
  useEffect(() => {
    let cancelled = false, frame = 0, lastSent = -Infinity;
    frame = requestAnimationFrame((started) => {
      const dark = page === "explore" || resolvedTheme === "dark";
      const shell = document.querySelector("[data-app-shell]");
      const color = page === "explore" ? "#0b131d" : nativeColor(shell ? getComputedStyle(shell).backgroundColor : "", dark ? "#101119" : "#fafaf7");
      const from = appliedColor || color;
      const rgb = (hex: string) => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));
      const a = rgb(from), b = rgb(color);
      const tick = (now: number) => {
        if (cancelled) return;
        const t = reduced || from === color ? 1 : Math.min(1, (now - started) / 320);
        if (now - lastSent >= 40 || t === 1) {
          lastSent = now;
          const eased = t * t * (3 - 2 * t);
          const channels = a.map((v, i) => Math.round(v + (b[i] - v) * eased));
          const hex = "#" + channels.map(v => v.toString(16).padStart(2, "0")).join("");
          // Native caption readability follows the intermediate painted RGB too.
          const captionDark = t === 1 ? dark : channels.reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0) < 145;
          pending = pending.then(async () => {
            if (cancelled) return;
            appliedColor = hex;
            await WindowAppearance.apply(captionDark, hex);
          }).catch(() => { console.warn("[hoard:titlebar] native appearance unavailable"); });
        }
        if (t < 1) frame = requestAnimationFrame(tick);
      };
      tick(started);
    });
    return () => { cancelled = true; cancelAnimationFrame(frame); };
  }, [page, resolvedTheme, reduced]);
}
