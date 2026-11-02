import { useEffect, useRef, useState } from "react";
import { animate, useReducedMotion } from "motion/react";
import { useTranslation } from "react-i18next";
import { NumberTicker } from "@/components/ui/number-ticker";
import { tickerTransition } from "@/lib/motion";

export function LibraryNumber({ value: input }: { value: number }) {
  const value = Number.isFinite(input) ? input : 0;
  const { i18n } = useTranslation("library");
  const reduced = useReducedMotion();
  const host = useRef<HTMLSpanElement>(null);
  const current = useRef(value);
  const [initial] = useState(value);
  const [display, setDisplay] = useState(value);
  const lastUpdate = useRef(0);
  useEffect(() => {
    const timer = setTimeout(() => { lastUpdate.current = Date.now(); setDisplay(value); }, Math.max(0, 1000 - (Date.now() - lastUpdate.current)));
    return () => clearTimeout(timer);
  }, [value]);
  useEffect(() => {
    const element = host.current?.firstElementChild;
    if (!element) return;
    const format = new Intl.NumberFormat(i18n.language);
    // The registry ticker fixes locale and spring settings; the host supplies the product transition.
    const animation = animate(Number.isFinite(current.current) ? current.current : 0, display, { ...tickerTransition, duration: reduced ? 0 : tickerTransition.duration,
      onUpdate(latest) { current.current = latest; element.textContent = format.format(Math.round(latest)); },
    });
    return () => animation.stop();
  }, [display, i18n.language, reduced]);
  return <span ref={host} className="tabular-nums"><NumberTicker value={initial} startValue={initial} className="text-inherit tracking-normal dark:text-inherit" /></span>;
}
