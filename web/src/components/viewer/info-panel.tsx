import { useTranslation } from "react-i18next";
import { Clapperboard, Images, Music2 } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import type { LibraryEntry, MediaResult } from "@/lib/library";
import { formatWhen } from "@/lib/time";
import { formatClock } from "@/lib/player";

function Caption({ text }: { text: string }) {
  return <p className="select-text cursor-text text-sm leading-relaxed break-words whitespace-pre-wrap text-viewer-foreground/85">
    {text.split(/(#[^\s#]+)/g).map((part, i) => part.startsWith("#") ? <span key={i} className="font-medium text-viewer-foreground">{part}</span> : part)}
  </p>;
}

function Chip({ icon: Icon, children }: { icon: typeof Music2; children: React.ReactNode }) {
  return <span className="inline-flex max-w-full items-center gap-1.5 rounded-full bg-viewer-control px-2.5 py-1 text-xs text-viewer-muted"><Icon className="size-3.5 shrink-0" /><span className="truncate">{children}</span></span>;
}

/** InfoPanel shows who made an item, when, and what it says. */
export function InfoPanel({ entry, media }: { entry: LibraryEntry; media: MediaResult | null }) {
  const { t, i18n } = useTranslation("library");
  const author = media?.author || entry.author;
  const avatar = entry.authorAvatar || media?.authorAvatar;
  const published = media?.publishedAt || entry.publishedAt;
  const desc = media?.desc || entry.title;
  const count = media?.images.length || entry.imageCount;
  return <div data-testid="viewer-info" className="flex h-full min-h-0 flex-col">
    <div className="flex items-center gap-3 px-6 pt-6 pb-5">
      <Avatar key={avatar} className="size-11 ring-1 ring-viewer-border">
        {avatar && <AvatarImage src={avatar} alt="" className="object-cover" />}
        <AvatarFallback className="bg-viewer-control text-base font-medium text-viewer-foreground">{(author || "?").slice(0, 1).toUpperCase()}</AvatarFallback>
      </Avatar>
      <div className="min-w-0">
        <p className="truncate text-[15px] font-semibold text-viewer-foreground">{author ? `@${author}` : t("viewer.unknownAuthor")}</p>
        {published > 0 && <p className="mt-0.5 text-xs text-viewer-muted">{formatWhen(published, i18n.language)}</p>}
      </div>
    </div>
    <div className="mx-6 h-px bg-viewer-border" />
    <div data-scroll-area className="scrollbar-none min-h-0 flex-1 overflow-y-auto px-6 py-5">
      {desc ? <Caption text={desc} /> : <p className="text-sm text-viewer-muted">{t("grid.untitled")}</p>}
      <div className="mt-5 flex flex-wrap gap-2">
        {entry.kind === "gallery"
          ? <Chip icon={Images}>{t("viewer.galleryCount", { count })}</Chip>
          : <Chip icon={Clapperboard}>{entry.durationMs > 0 ? `${t("viewer.video")} · ${formatClock(entry.durationMs / 1000)}` : t("viewer.video")}</Chip>}
        {entry.kind === "gallery" && media?.music.title && <Chip icon={Music2}>{media.music.author ? `${media.music.title} · ${media.music.author}` : media.music.title}</Chip>}
      </div>
    </div>
  </div>;
}
