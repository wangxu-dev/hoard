import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { EntryCard } from "@/components/library-grid";
import { Spinner } from "@/components/ui/spinner";
import { libraryEntry, type PanelView } from "./view-types";
import { nearPageEnd, SnapshotPages } from "./snapshot-pages";

// Shares Hoard's real card and virtualizer. This pager slices one immutable native snapshot.
export function SnapshotWorkList({ view, hidden }: { view: PanelView; hidden: boolean }) {
  const scroll=useRef<HTMLDivElement>(null), savedScroll=useRef(0), frame=useRef(0);
  const [revision,setRevision]=useState(0),[loading,setLoading]=useState(false),[width,setWidth]=useState(288);
  const pager=useMemo(()=>new SnapshotPages(view.works),[view.key]);
  const entries=useMemo(()=>new Map(pager.items.map(work=>[work.key,libraryEntry(work)])),[pager]);
  const visible=pager.visible;
  const rowHeight=(Math.max(60,(width-6)/2))*4/3+70;
  const virtual=useVirtualizer({count:Math.ceil(visible.length/2),getScrollElement:()=>scroll.current,estimateSize:()=>rowHeight,overscan:2,getItemKey:index=>visible[index*2]?.key??index,initialRect:{width:320,height:600}});
  const request=useCallback(async()=>{if(hidden||pager.loading||!pager.more)return;setLoading(true);const ok=await pager.next();if(ok){setRevision(n=>n+1);setLoading(false);}},[pager,hidden]);
  const maybeLoad=useCallback(()=>{const el=scroll.current;if(!el||hidden)return;if(nearPageEnd(el.scrollTop,el.clientHeight,virtual.getTotalSize()))void request();},[hidden,request,virtual,revision]);
  const schedule=useCallback(()=>{cancelAnimationFrame(frame.current);frame.current=requestAnimationFrame(maybeLoad);},[maybeLoad]);
  useEffect(()=>{const el=scroll.current;if(!el)return;const resize=new ResizeObserver(()=>{setWidth(Math.max(120,el.clientWidth-32));schedule();});resize.observe(el);return()=>resize.disconnect();},[schedule]);
  useLayoutEffect(()=>{virtual.measure();if(!hidden&&scroll.current)scroll.current.scrollTop=savedScroll.current;schedule();},[rowHeight,hidden,revision,schedule,virtual]);
  useEffect(()=>()=>{pager.cancel();cancelAnimationFrame(frame.current);},[pager]);
  return <div ref={scroll} hidden={hidden} aria-busy={loading} onScroll={()=>{if(hidden)return;savedScroll.current=scroll.current?.scrollTop??0;schedule();}} className="scrollbar-none min-h-0 flex-1 overflow-y-auto px-4">
    <div className="relative mt-4" style={{height:virtual.getTotalSize()}}>{virtual.getVirtualItems().map(row=><div key={row.key} className="absolute top-0 left-0 grid w-full grid-cols-2 gap-1.5 pb-1.5" style={{transform:`translateY(${row.start}px)`}}>{visible.slice(row.index*2,row.index*2+2).map(work=><EntryCard key={work.key} readOnlyPreview entry={entries.get(work.key)!} onOpen={()=>view.openWork(work.key)} />)}</div>)}</div>
    {loading&&<div role="status" aria-label="载入中" className="flex h-9 items-center justify-center text-muted-foreground"><Spinner className="size-4" /></div>}
  </div>;
}
