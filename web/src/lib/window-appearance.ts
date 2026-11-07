import { useEffect } from "react";
import { useTheme } from "next-themes";
import { WindowAppearance } from "@mygo";

let pending = Promise.resolve();
// Resolve the actually painted CSS color (including OKLCH) into native sRGB.
function nativeColor(css: string, fallback: string): string {
 const ctx = document.createElement("canvas").getContext("2d", {willReadFrequently:true});
 if (!ctx) return fallback;
 ctx.fillStyle=fallback; ctx.fillStyle=css; ctx.fillRect(0,0,1,1);
 const rgba=ctx.getImageData(0,0,1,1).data;
 return "#"+Array.from(rgba.slice(0,3),v=>v.toString(16).padStart(2,"0")).join("");
}
export function useWindowAppearance(page: string) {
 const {resolvedTheme}=useTheme();
 useEffect(()=>{
  const frame=requestAnimationFrame(()=>{
   const dark=page==="explore" || resolvedTheme==="dark";
   const shell=document.querySelector("[data-app-shell]");
   const color=page==="explore"?"#0b131d":nativeColor(shell?getComputedStyle(shell).backgroundColor:"",dark?"#101119":"#fafaf7");
   pending=pending.then(()=>WindowAppearance.apply(dark,color)).catch(()=>{console.warn("[hoard:titlebar] native appearance unavailable");});
  });
  return ()=>cancelAnimationFrame(frame);
 },[page,resolvedTheme]);
}
