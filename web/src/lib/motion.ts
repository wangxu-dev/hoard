import type { Transition, Variants } from "motion/react";

function duration(name: string, fallback: number): number {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const milliseconds = Number.parseFloat(value);
  return Number.isFinite(milliseconds) ? milliseconds / 1000 : fallback;
}

export const pageTransition: Transition = {
  duration: duration("--duration-base", 0.2),
  ease: "easeOut",
};

export const fastTransition: Transition = { duration: duration("--duration-fast", 0.12), ease: "easeOut" };
export const tickerTransition = { duration: duration("--duration-slow", 0.32), ease: "easeOut" as const };
export const pulseTransition: Transition = { duration: duration("--duration-pulse", 1.2), ease: "easeInOut", repeat: Infinity, repeatType: "reverse" };

export const pageVariants: Variants = {
  initial: { opacity: 0, y: 8 },
  animate: { opacity: 1, y: 0, transition: pageTransition },
  exit: { opacity: 0, y: -4, transition: pageTransition },
};
