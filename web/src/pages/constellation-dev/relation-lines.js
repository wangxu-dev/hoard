import H from './core.js';
const strength=w=>Math.sqrt(Math.max(0,Math.min(1,w)));
export function relationStyle(weight,local=false){const s=strength(weight);return {alpha:local?.19+.53*s:.035+.075*s,width:local?.45+1.35*s:.35+.45*s};}
// O(E) at mount, bounded O(budget+local degree) per frame. No spatial inference.
export class RelationLayer{
 constructor(works,byKey,budget=42){this.byKey=byKey;const slots=new Array(budget);for(const a of works)for(const n of a.neighbors||[]){const b=byKey.get(n.key);if(!b||a.key>=b.key||!(n.weight>0))continue;const rank=H.hash(a.key+'|'+b.key),slot=rank%budget;if(!slots[slot]||rank<slots[slot].rank)slots[slot]={a,b,weight:n.weight,rank};}this.background=slots.filter(Boolean);}
 frame(selected,hover){const edges=new Map(),weights=new Map();const add=(a,b,weight,local)=>{const key=[a.key,b.key].sort().join('|'),prior=edges.get(key);if(!prior||local)edges.set(key,{a,b,weight,local});};for(const e of this.background)add(e.a,e.b,e.weight,false);
  for(const [work,cap] of [[selected,36],[hover,24]])if(work){weights.set(work.key,1);for(const n of (work.neighbors||[]).slice(0,cap)){const b=this.byKey.get(n.key);if(!b||!(n.weight>0))continue;weights.set(b.key,Math.max(weights.get(b.key)||0,n.weight));add(work,b,n.weight,true);}}
  return {edges:[...edges.values()],weights};
 }
 draw(ctx,frame,project,rendered,width,height,opacity=1){let drawn=0,local=0,weak=0;for(const e of frame.edges){if(!rendered.has(e.a.key)||!rendered.has(e.b.key))continue;const a=project(e.a.pos),b=project(e.b.pos);if((a.x<0&&b.x<0)||(a.y<0&&b.y<0)||(a.x>width&&b.x>width)||(a.y>height&&b.y>height))continue;const style=relationStyle(e.weight,e.local),depth=Math.max(.25,Math.min(1,(Math.min(a.depth,b.depth)+1.6)/2.4));ctx.globalAlpha=style.alpha*depth*opacity;ctx.strokeStyle=e.local?'#a2e4df':'#659ba8';ctx.lineWidth=style.width;ctx.beginPath();ctx.moveTo(a.x,a.y);ctx.lineTo(b.x,b.y);ctx.stroke();drawn++;if(e.local)local++;if(e.weight<.25)weak++;}ctx.globalAlpha=1;return {drawn,local,weak,budget:this.background.length+60};}
}
