import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Download, MessageSquareWarning, RefreshCw } from "lucide-react";
import { siGithub } from "simple-icons";
import { toast } from "sonner";
import { setLanguage } from "@/i18n";
import { Updates } from "@/lib/api";
import { REPO_URL, VERSION, useUpdate } from "@/lib/updates";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

/** A titled group of rows on one card. */
function Section({ title, children }: { title: string; children: ReactNode }) {
  return <section className="flex flex-col gap-2">
    <h2 className="px-1 text-xs font-medium text-muted-foreground">{title}</h2>
    <div className="divide-y overflow-hidden rounded-lg border bg-card text-card-foreground">{children}</div>
  </section>;
}

/** One setting: what it is on the left, its control or value on the right. */
function Row({ testId, title, description, children }: { testId: string; title: string; description: string; children: ReactNode }) {
  return <div data-testid={testId} className="flex min-h-16 min-w-0 items-center justify-between gap-6 px-4 py-3">
    <div className="min-w-0">
      <p className="text-sm font-medium">{title}</p>
      <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
    </div>
    <div className="shrink-0">{children}</div>
  </div>;
}

function SiGithub() {
  return <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d={siGithub.path} /></svg>;
}

/** An icon-only button that names itself in a tooltip. */
function IconButton({ label, variant = "ghost", children, ...props }: { label: string; variant?: "ghost" | "default"; children: ReactNode; onClick: () => void; disabled?: boolean }) {
  return <Tooltip>
    <TooltipTrigger asChild><Button variant={variant} size="icon-sm" aria-label={label} {...props}>{children}</Button></TooltipTrigger>
    <TooltipContent>{label}</TooltipContent>
  </Tooltip>;
}

// Laid out like the library: a 56px bar on top, then one centered column.
export function SettingsPage() {
  const { t, i18n } = useTranslation("settings");
  const { info, available, check } = useUpdate();
  const [checking, setChecking] = useState(false);
  const open = (url: string) => { void Updates.open(url).catch(() => toast.error(t("update.openFailed"))); };
  const runCheck = async () => {
    setChecking(true);
    try {
      const next = await check();
      if (next.error) toast.error(t(`update.error.${next.error}`, { defaultValue: t("update.error.OFFLINE") }));
      else if (next.status !== "available") toast.success(t("update.current"));
    } catch {
      toast.error(t("update.error.OFFLINE"));
    } finally {
      setChecking(false);
    }
  };
  return <main data-testid="settings" className="min-h-full min-w-0 flex-1 bg-background">
    <header className="flex h-14 items-center px-6">
      <h1 className="text-[15px] font-semibold">{t("title")}</h1>
    </header>
    <div data-testid="settings-content" className="mx-auto flex w-full max-w-[680px] flex-col gap-8 px-6 pt-6 pb-16">
      <Section title={t("general")}>
        <Row testId="settings-language-row" title={t("language.title")} description={t("language.description")}>
          <Select value={i18n.language} onValueChange={setLanguage}>
            <SelectTrigger className="w-36" aria-label={t("language.title")}><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="zh-CN">{t("language.zhCN")}</SelectItem>
              <SelectItem value="en">{t("language.en")}</SelectItem>
            </SelectContent>
          </Select>
        </Row>
      </Section>
      <section className="flex flex-col gap-2">
        <h2 className="px-1 text-xs font-medium text-muted-foreground">{t("about")}</h2>
        <div className="divide-y overflow-hidden rounded-lg border bg-card text-card-foreground">
          <Row testId="settings-about-row" title="Hoard" description={available ? t("update.available", { version: info?.latest }) : t("app.description", { version: VERSION })}>
            <div className="flex items-center gap-1">
              <IconButton label={t("github")} onClick={() => open(REPO_URL)}><SiGithub /></IconButton>
              <IconButton label={t("issues")} onClick={() => open(`${REPO_URL}/issues`)}><MessageSquareWarning /></IconButton>
              {available
                ? <IconButton label={t("update.download")} variant="default" onClick={() => open(info!.url)}><Download /></IconButton>
                : <IconButton label={t("update.check")} disabled={checking} onClick={() => { void runCheck(); }}><RefreshCw className={checking ? "animate-spin" : undefined} /></IconButton>}
            </div>
          </Row>
          {available && info?.notes && <div data-testid="settings-update-notes" className="max-h-56 overflow-y-auto px-4 py-3 text-xs whitespace-pre-wrap text-muted-foreground">{info.notes}</div>}
        </div>
        <p className="px-1 text-xs text-muted-foreground">{t("disclaimer")}</p>
      </section>
    </div>
  </main>;
}
