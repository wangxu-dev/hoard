import { useTranslation } from "react-i18next";
import { useState } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { SnapshotWorkList } from "./work-list";
import { detailAvatarURL } from "./avatar";
import type { PanelView } from "./view-types";

function CachedImage({ src, alt, className, report }: { src: string; alt: string; className: string; report: PanelView["imageStatus"] }) {
  const [failed, setFailed] = useState("");
  const [loaded, setLoaded] = useState("");
  if (!src || failed === src) return null;
  return <img src={src} alt={alt} loading="lazy" referrerPolicy="no-referrer" className={`${className} transition-opacity duration-200 ${loaded === src ? "opacity-100" : "opacity-0"}`}
    onLoad={() => { setLoaded(src); report(src, 1); }} onError={() => { setFailed(src); report(src, -1); }} />;
}

function AuthorIdentity({author,avatar,report}:{author:string;avatar:string;report:PanelView["imageStatus"]}){
 const src=detailAvatarURL(avatar),[loaded,setLoaded]=useState(""),[failed,setFailed]=useState("");
 const state=!src?"missing":failed===src?"failed":loaded===src?"ready":"loading";
 return <div data-author-status data-state={state} className="flex items-center gap-3 px-4 pt-4 text-sm">
  <div className="relative size-9 shrink-0 overflow-hidden rounded-full bg-white/10">
   <span className="absolute inset-0 grid place-items-center text-xs text-muted-foreground">{author.slice(0,1)||"人"}</span>
   {src&&failed!==src&&<img src={src} alt="作者头像" referrerPolicy="no-referrer" className={`absolute inset-0 size-full object-cover ${loaded===src?"opacity-100":"opacity-0"}`} onLoad={()=>{setLoaded(src);report(src,1);}} onError={()=>{setFailed(src);report(src,-1);}}/>}
  </div>
  <div className="min-w-0"><span className="block leading-snug break-words text-popover-foreground">{author||"未知作者"}</span>{(state==="missing"||state==="failed")&&<span className="block pt-1 text-xs text-muted-foreground">头像暂不可用</span>}</div>
 </div>;
}

export function ContentDetails({ view }: { view: PanelView | null }) {
  const { t } = useTranslation("explore");
  if (!view) return null;
  const { work } = view;
  return <aside data-explore-details aria-label={work ? "收藏详情" : view.label} className="explore-detail-surface absolute inset-y-4 right-4 z-30 flex w-80 max-w-[calc(100%-2rem)] flex-col overflow-hidden rounded-xl border border-border bg-popover/95 text-popover-foreground shadow-lg backdrop-blur-(--glass-blur)">
    <div className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border px-4">
      {work ? <Badge variant="outline">{work.source}</Badge> : <h2 className="min-w-0 truncate text-sm font-medium">{view.label}</h2>}
      <Button variant="ghost" size="icon" onClick={view.back} aria-label="关闭"><X /></Button>
    </div>
    {!work && <SnapshotWorkList key={view.key} view={view} hidden={false} />}
    {work && <div className="scrollbar-none min-h-0 max-h-[55%] shrink-0 overflow-y-auto">
      <div className="pb-4">
        {work ? <>
          <CachedImage src={work.cover} alt="收藏封面" className="mx-auto mt-4 max-h-44 w-[calc(100%-2rem)] rounded-lg object-cover" report={view.imageStatus} />
          <AuthorIdentity key={work.key} author={work.author || work.source} avatar={work.avatar} report={view.imageStatus}/>
          {work.outsideTimeRange && <p role="status" className="px-4 pt-3 text-xs text-muted-foreground">{t("timeline.outside")}</p>}
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
