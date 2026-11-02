"use client";

import { useEffect, useState } from "react";
import { Languages } from "lucide-react";
import { useReducedMotion } from "motion/react";
import { useTheme } from "next-themes";
import { Button } from "@app-ui/button";
import { AnimatedThemeToggler } from "@app-ui/animated-theme-toggler";
import { dictionaries, type Language } from "@/lib/content";

export function Preferences({
  language,
  privacy = false,
}: {
  language: Language;
  privacy?: boolean;
}) {
  const { resolvedTheme, setTheme } = useTheme();
  const reducedMotion = useReducedMotion();
  const [mounted, setMounted] = useState(false);
  const isDark = mounted && resolvedTheme === "dark";
  useEffect(() => { setMounted(true); }, []);
  const text = dictionaries[language].controls;
  const otherPath =
    (language === "en" ? "/" : "/en/") + (privacy ? "privacy/" : "");
  return (
    <div className="flex items-center gap-1">
      <Button variant="ghost" size="icon-sm" asChild>
        <a
          href={otherPath}
          hrefLang={language === "en" ? "zh-CN" : "en"}
          aria-label={text.language}
        >
          <Languages />
        </a>
      </Button>
      <AnimatedThemeToggler
        className="flex size-8 items-center justify-center rounded-md hover:bg-sidebar-accent [&_svg]:size-4"
        theme={isDark ? "dark" : "light"}
        onThemeChange={setTheme}
        duration={reducedMotion ? 0 : undefined}
        aria-label={isDark ? text.light : text.dark}
      />
    </div>
  );
}
