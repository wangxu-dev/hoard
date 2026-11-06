import { useState } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { SnapshotWorkList } from "./work-list";
import type { PanelView } from "./view-types";

function CachedImage({ src, alt, className, report }: { src: string; alt: string; className: string; report: PanelView["imageStatus"] }) {
  const [failed, setFailed] = useState("");
  const [loaded, setLoaded] = useState("");
  if (!src || failed === src) return null;
  return <img src={src} alt={alt} loading="lazy" referrerPolicy="no-referrer" className={`${className} transition-opacity duration-200 ${loaded === src ? "opacity-100" : "opacity-0"}`}
    onLoad={() => { setLoaded(src); report(src, 1); }} onError={() => { setFailed(src); report(src, -1); }} />;
}

export function ContentDetails({ view }: { view: PanelView | null }) {
  if (!view) return null;
  const { work } = view;
  return <aside aria-label={work ? "收藏详情" : view.label} className="absolute inset-y-4 right-4 z-30 flex w-80 max-w-[calc(100%-2rem)] flex-col overflow-hidden rounded-xl border border-border bg-popover/95 text-popover-foreground shadow-lg backdrop-blur-(--glass-blur)">
    <div className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border px-4">
      {work ? <Badge variant="outline">{work.source}</Badge> : <h2 className="min-w-0 truncate text-sm font-medium">{view.label}</h2>}
      <Button variant="ghost" size="icon" onClick={view.back} aria-label="关闭"><X /></Button>
    </div>
    {!work && <SnapshotWorkList key={view.key} view={view} hidden={false} />}
    {work && <div className="scrollbar-none min-h-0 max-h-[55%] shrink-0 overflow-y-auto">
      <div className="pb-4">
        {work ? <>
          <CachedImage src={work.cover} alt="收藏封面" className="mx-auto mt-4 max-h-44 w-[calc(100%-2rem)] rounded-lg object-cover" report={view.imageStatus} />
          <div className="flex items-center gap-2 px-4 pt-4 text-sm text-muted-foreground">
            <CachedImage src={work.avatar} alt="" className="size-7 rounded-full object-cover" report={view.imageStatus} />
            <span className="min-w-0 truncate">{work.author || work.source}</span>
          </div>
          <p className="select-text px-4 pt-3 text-sm leading-relaxed break-words whitespace-pre-wrap text-popover-foreground">{work.title}</p>

        </> : null}
      </div>
    </div>}
    {work && <>
      <h2 className="shrink-0 border-t border-border px-4 pt-4 text-sm font-medium">{view.works.length ? `相关作品 · ${view.works.length}` : "暂无关联作品"}</h2>
      <SnapshotWorkList key={view.key} view={view} hidden={false} />
    </>}
  </aside>;
}
