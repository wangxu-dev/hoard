import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import Markdown from "react-markdown";
import { Download, MessageSquareWarning, RefreshCw } from "lucide-react";
import { siGithub } from "simple-icons";
import { toast } from "sonner";
import { setLanguage } from "@/i18n";
import { Updates } from "@/lib/api";
import { RELEASE_NOTES, REPO_URL, VERSION, useUpdate } from "@/lib/updates";
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
  const { info, available, check, transfer, download, restart } = useUpdate();
  const [starting, setStarting] = useState(false);
  const downloading = transfer?.status === "downloading";
  const ready = transfer?.status === "ready";
  const failed = transfer?.status === "error";
  const runUpdate = async () => {
    setStarting(true);
    try { if (ready) await restart(); else await download(); }
    catch { toast.error(t(ready ? "update.restartFailed" : "update.downloadFailed")); }
    finally { setStarting(false); }
  };
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
          <Row testId="settings-about-row" title="Hoard" description={t("app.description", { version: VERSION })}>
            <div className="flex items-center gap-1">
              <Button data-testid="settings-update-action" variant={available ? "default" : "outline"} size="sm" disabled={checking || starting || downloading} onClick={() => { if (!available) void runCheck(); else if (transfer?.enabled) void runUpdate(); else open(`${REPO_URL}/releases`); }}>
                {available && !ready ? <Download aria-hidden="true" /> : <RefreshCw aria-hidden="true" className={checking ? "animate-spin" : undefined} />}
                {t(!available ? "update.check" : ready ? "update.restart" : downloading ? "update.downloading" : failed ? "update.retry" : transfer?.enabled ? "update.download" : "update.manual")}
                {downloading && transfer.total > 0 && <span className="tabular-nums">{Math.min(100, Math.floor(transfer.downloaded / transfer.total * 100))}%</span>}
              </Button>
            </div>
          </Row>

          {available && <div data-testid="settings-update-panel" className="flex min-w-0 flex-col gap-3 px-4 py-3">
            <div className="flex items-center gap-2 text-sm">
              <span className="font-medium">{t("update.version", { version: transfer?.version || info?.latest })}</span>
              <span className="text-xs text-brand">{t(ready ? "update.readyBadge" : "update.newBadge")}</span>
            </div>
            <div data-testid="settings-update-notes" className="max-h-40 overflow-y-auto text-xs leading-relaxed text-muted-foreground [&_h3]:mb-1 [&_h3]:mt-3 [&_h3]:font-medium [&_h3]:text-foreground [&_h3:first-child]:mt-0 [&_ul]:list-disc [&_ul]:space-y-1 [&_ul]:pl-4 [&_p]:mb-1">
              <Markdown allowedElements={["p", "h3", "ul", "ol", "li", "strong", "em", "code", "br"]} unwrapDisallowed>{info?.notes || ""}</Markdown>
            </div>
            {failed && <p className="text-xs text-muted-foreground" role="status">{t("update.downloadFailed")}</p>}
          </div>}
          {!available && RELEASE_NOTES && <div data-testid="settings-update-notes" className="max-h-56 overflow-y-auto px-4 py-3 text-xs leading-relaxed text-muted-foreground [&_h3]:mb-1 [&_h3]:mt-3 [&_h3]:font-medium [&_h3]:text-foreground [&_h3:first-child]:mt-0 [&_ul]:list-disc [&_ul]:space-y-1 [&_ul]:pl-4 [&_p]:mb-1"><Markdown allowedElements={["p", "h3", "ul", "ol", "li", "strong", "em", "code", "br"]} unwrapDisallowed>{RELEASE_NOTES}</Markdown></div>}
        </div>
          <div data-testid="settings-about-footer" className="flex items-center justify-end gap-1 text-muted-foreground">
            <div className="flex shrink-0 items-center gap-1 text-muted-foreground">
              <IconButton label={t("github")} onClick={() => open(REPO_URL)}><SiGithub /></IconButton>
              <IconButton label={t("issues")} onClick={() => open(`${REPO_URL}/issues`)}><MessageSquareWarning /></IconButton>
            </div>
          </div>
      </section>
    </div>
  </main>;
}
