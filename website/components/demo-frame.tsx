"use client";

import { useEffect, useRef } from "react";
import { useTheme } from "next-themes";
import { dictionaries, type Language } from "@/lib/content";

export function DemoFrame({
  language,
  collection = false,
  className,
}: {
  language: Language;
  collection?: boolean;
  className: string;
}) {
  const frame = useRef<HTMLIFrameElement>(null);
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
      ref={frame}
      src={
        "/demo/index.html?lang=" +
        language +
        (collection ? "&view=collection" : "")
      }
      title={
        dictionaries[language].features[
          collection ? "collectionLabel" : "previewLabel"
        ]
      }
      onLoad={syncTheme}
      className={className}
      tabIndex={collection ? -1 : undefined}
    />
  );
}
