"use client";

import { useEffect, useState } from "react";
import { useReducedMotion } from "motion/react";
import { BlurFade } from "@/components/ui/blur-fade";

export function Reveal({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const reducedMotion = useReducedMotion();
  const [duration, setDuration] = useState(0);
  useEffect(() => {
    setDuration(
      parseFloat(
        getComputedStyle(document.documentElement).getPropertyValue(
          "--duration-slow",
        ),
      ) / 1000,
    );
  }, []);
  if (reducedMotion) return <div className={className}>{children}</div>;
  return (
    <BlurFade
      className={className}
      data-reveal
      inView
      duration={duration}
      blur="0px"
      offset={4}
    >
      {children}
    </BlurFade>
  );
}
