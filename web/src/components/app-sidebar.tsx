import type { CSSProperties, ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { Library, Settings } from "lucide-react";
import { useTheme } from "next-themes";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { useUpdate } from "@/lib/updates";
import { AnimatedThemeToggler } from "@/components/ui/animated-theme-toggler";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export type Page = "library" | "settings";

interface AppSidebarProps {
  page: Page;
  onPageChange: (page: Page) => void;
  children: ReactNode;
}

const navigation = [{ id: "library" as const, icon: Library }];
// Rail buttons keep the same left padding when collapsed, so their icons never move.
const railButton = "h-10 gap-3 px-3 group-data-[collapsible=icon]:size-10! group-data-[collapsible=icon]:px-3!";

const token = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
// Every sidebar tooltip keeps the same gap from its button.
const TIP_OFFSET = 8;

export function AppSidebar({ page, onPageChange, children }: AppSidebarProps) {
  const { t } = useTranslation("common");
  const { resolvedTheme, setTheme } = useTheme();
  const [open, setOpen] = useState(() => localStorage.getItem("hoard-sidebar") !== "false");
  const isDark = resolvedTheme === "dark";
  const { available, info } = useUpdate();
  const { t: updateText } = useTranslation("settings");
  const notified = useRef("");
  useEffect(() => {
    if (!available || !info || notified.current === info.latest) return;
    notified.current = info.latest;
    toast(updateText("update.available", { version: info.latest }), {
      id: "available-update",
      action: { label: updateText("update.view"), onClick: () => onPageChange("settings") },
    });
  }, [available, info, onPageChange, updateText]);

  const updateOpen = (next: boolean) => {
    setOpen(next);
    localStorage.setItem("hoard-sidebar", String(next));
  };

  return (
    <SidebarProvider open={open} onOpenChange={updateOpen} style={{ "--sidebar-width": token("--sidebar-width"), "--sidebar-width-icon": token("--sidebar-width-icon") } as CSSProperties}>
      <Sidebar collapsible="icon" className="bg-sidebar">
        <SidebarHeader className="h-12 border-0 px-2">
          {/* Everything stays anchored while the width animates: the title fades and the
              toggle rides the right edge until it sits centered in the collapsed rail. */}
          <div className="flex min-w-0 items-center justify-between">
            <span className="min-w-0 truncate pl-3 text-sm group-data-[collapsible=icon]:pl-0 font-semibold tracking-tight transition-opacity duration-200 group-data-[collapsible=icon]:opacity-0">{t("appName")}</span>
            <Tooltip>
              <TooltipTrigger asChild>
                <SidebarTrigger aria-label={t(open ? "sidebar.collapse" : "sidebar.expand")} className="size-10 shrink-0 rounded-md hover:bg-sidebar-accent hover:text-sidebar-accent-foreground" />
              </TooltipTrigger>
              <TooltipContent side={open ? "bottom" : "right"} sideOffset={TIP_OFFSET}>
                {t(open ? "sidebar.collapse" : "sidebar.expand")}
              </TooltipContent>
            </Tooltip>
          </div>
        </SidebarHeader>
        <SidebarContent className="bg-sidebar">
          <SidebarGroup className="px-2 pt-2 pb-4">
            <SidebarGroupContent>
              <SidebarMenu>
                {navigation.map(({ id, icon: Icon }) => (
                    <SidebarMenuItem key={id}>
                    <SidebarMenuButton
                      isActive={page === id}
                      onClick={() => onPageChange(id)}
                      aria-current={page === id ? "page" : undefined}
                      tooltip={{ children: t(`sidebar.${id}`), sideOffset: TIP_OFFSET }}
                      className={railButton}
                    >
                      <Icon aria-hidden="true" />
                      <span>{t(`sidebar.${id}`)}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter className="border-0 bg-sidebar p-2">
          <div className="flex w-full min-w-0 items-center">
            <SidebarMenuButton
              isActive={page === "settings"}
              onClick={() => onPageChange("settings")}
              aria-current={page === "settings" ? "page" : undefined}
              tooltip={{ children: t("sidebar.settings"), sideOffset: TIP_OFFSET }}
              className={`relative min-w-0 flex-1 ${railButton}`}
            >
              <Settings aria-hidden="true" />
              {/* Sits on the icon's top-right corner; the icon is the button's direct child so its size rule applies. */}
              {available && <span data-testid="update-dot" aria-label={t("sidebar.update")} className="absolute top-2.5 left-6 size-2 rounded-full bg-brand" />}
              <span>{t("sidebar.settings")}</span>
            </SidebarMenuButton>
            <div className="ml-1 w-8 shrink-0 overflow-hidden transition-[width,margin,opacity] duration-200 ease-(--ease-out) group-data-[collapsible=icon]:ml-0 group-data-[collapsible=icon]:w-0 group-data-[collapsible=icon]:opacity-0">
              <SidebarMenuButton asChild onClickCapture={() => console.debug("[hoard:ui] theme click")} tooltip={{ children: t("theme.toggle"), sideOffset: TIP_OFFSET }} className="size-8 shrink-0 justify-center p-0">
                <span>
                  <AnimatedThemeToggler
                    className="flex size-8 items-center justify-center rounded-md hover:bg-sidebar-accent [&_svg]:size-4"
                    theme={isDark ? "dark" : "light"}
                    onThemeChange={(next) => { console.debug("[hoard:ui] theme changed", next); setTheme(next); }}
                    aria-label={t("theme.toggle")}
                  />
                </span>
              </SidebarMenuButton>
            </div>
          </div>
        </SidebarFooter>
        <SidebarRail aria-label={t(open ? "sidebar.collapse" : "sidebar.expand")} title={t(open ? "sidebar.collapse" : "sidebar.expand")} />
      </Sidebar>
      {children}
    </SidebarProvider>
  );
}
