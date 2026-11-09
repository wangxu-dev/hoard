"use client";

import { useEffect, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { dictionaries, type Language } from "@/lib/content";

let visitSeed: number | undefined;

export function DemoFrame({
  language,
  collection = false,
  exploration = false,
  className,
}: {
  language: Language;
  collection?: boolean;
  exploration?: boolean;
  className: string;
}) {
  const frame = useRef<HTMLIFrameElement>(null);
  const [seed, setSeed] = useState<number | null>(null);
  useEffect(() => { setSeed(visitSeed ??= crypto.getRandomValues(new Uint32Array(1))[0]); }, []);
  const [visible, setVisible] = useState(collection);
  useEffect(() => {
    if (collection || visible || !frame.current) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) { setVisible(true); observer.disconnect(); }
    });
    observer.observe(frame.current);
    return () => observer.disconnect();
  }, [collection, visible]);
  const { resolvedTheme } = useTheme();
  const syncTheme = () =>
    frame.current?.contentWindow?.postMessage(
      {
        type: "hoard-preview-theme",
        theme: resolvedTheme === "dark" ? "dark" : "light",
      },
      location.origin,
    );
  useEffect(syncTheme, [resolvedTheme]);
  return (
    <iframe
      loading={collection ? "eager" : "lazy"}
      ref={frame}
      src={
        visible && seed !== null ? "/demo/index.html?seed=" + seed + "&lang=" +
        language +
        (collection ? "&view=collection" : exploration ? "&view=explore" : "") : undefined
      }
      title={
        exploration ? dictionaries[language].exploration.previewLabel : dictionaries[language].features[
          collection ? "collectionLabel" : "previewLabel"
        ]
      }
      onLoad={syncTheme}
      className={className}
      tabIndex={collection ? -1 : undefined}
    />
  );
}
