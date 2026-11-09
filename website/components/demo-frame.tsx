"use client";

import { useEffect, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { dictionaries, type Language } from "@/lib/content";

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
  const [visible, setVisible] = useState(!exploration);
  useEffect(() => {
    if (!exploration || visible || !frame.current) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) { setVisible(true); observer.disconnect(); }
    }, { rootMargin: "200px" });
    observer.observe(frame.current);
    return () => observer.disconnect();
  }, [exploration, visible]);
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
      loading={exploration ? "lazy" : "eager"}
      ref={frame}
      src={
        visible ? "/demo/index.html?lang=" +
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
