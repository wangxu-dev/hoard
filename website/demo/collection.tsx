import { useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import { EntryCard } from "@/components/library-grid";
import { Card } from "@/components/ui/card";
import { siTiktok, siBilibili, siYoutube, siXiaohongshu } from "simple-icons";
import type { LibraryEntry } from "@/lib/library";
import { AnimatedBeam } from "../components/ui/animated-beam";
import { demoEntries } from "./backend";

export function Collection() {
  const container = useRef<HTMLDivElement>(null);
  const left = useRef<HTMLDivElement>(null);
  const right = useRef<HTMLDivElement>(null);
  const center = useRef<HTMLDivElement>(null);
  const reducedMotion = useReducedMotion();
  const [entries] = useState(() => ["douyin", "bilibili"].map(source => {
    const pool = demoEntries.filter(entry => entry.accountIds.includes(`demo-${source}`));
    return pool[Math.floor(Math.random() * pool.length)];
  }));
  return (
    <div
      ref={container}
      className="relative flex h-dvh items-center justify-between gap-3 overflow-hidden bg-background px-5"
    >
      {[
        { ref: left, entry: entries[0], source: "douyin" },
        { ref: right, entry: entries[1], source: "bilibili" },
      ].map((item, index) => (
        <div
          key={item.source}
          ref={item.ref}
          className={`relative z-10 w-[clamp(96px,26vw,140px)] shrink-0 ${index ? "order-3 rotate-6" : "-rotate-6"}`}
        >
          <EntryCard entry={item.entry as LibraryEntry} onOpen={() => {}} />
        </div>
      ))}
      <div className="relative z-10 flex flex-col items-center gap-3">
        <Card ref={center} className="gap-0 rounded-md p-3 shadow-none">
          <img src="/icon.png" width="44" height="44" alt="" />
        </Card>
        <span className="text-sm font-semibold">Hoard</span>
      </div>
      <div
        className="pointer-events-none absolute inset-0 z-10"
        aria-hidden="true"
      >
        {[siTiktok, siBilibili, siYoutube, siXiaohongshu].map((icon, index) => (
          <svg
            key={icon.slug}
            viewBox="0 0 24 24"
            className={`absolute size-5 fill-current text-muted-foreground ${["left-[36%] top-4", "right-[35%] top-5", "bottom-3 left-[38%]", "bottom-4 right-[36%]"][index]}`}
          >
            <path d={icon.path} />
          </svg>
        ))}
      </div>
      {[left, right].map((from, index) => (
        <AnimatedBeam
          key={index}
          containerRef={container}
          fromRef={from}
          toRef={center}
          curvature={-20}
          pathColor="var(--border)"
          pathOpacity={1}
          gradientStartColor="var(--brand)"
          gradientStopColor="var(--brand)"
          duration={reducedMotion ? 0 : 5}
          repeat={reducedMotion ? 0 : Infinity}
          reverse={index === 1}
        />
      ))}
    </div>
  );
}
