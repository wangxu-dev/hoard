// Display leases affect only which real work covers are visible. Never positions or relations.
const exposure=new Map();let sessions=0;
function hash(s){let x=2166136261;for(let i=0;i<s.length;i++){x^=s.charCodeAt(i);x=Math.imul(x,16777619);}return (x>>>0)/4294967296;}
function remember(key,dt){const n=(exposure.get(key)||0)+dt;exposure.delete(key);exposure.set(key,n);if(exposure.size>8192)exposure.delete(exposure.keys().next().value);}
export class CoverDirector {
 constructor({seed,hold=6500,fade=1100,interval=1600}={}){this.seed=seed??`${++sessions}:${globalThis.crypto?.getRandomValues(new Uint32Array(1))[0]??Date.now()}`;this.hold=hold;this.fade=fade;this.interval=interval;this.items=new Map();this.lastAdmission=-Infinity;this.lastTick=null;this.lastMotion=null;this.motionCredit=0;this.failed=new Set();this.stats={admitted:0,retired:0,handoffs:0,distinct:0,midFade:0};this.seen=new Set();}
 score(p,width,height){const x=p.x/width,y=p.y/height,edge=Math.max(0,Math.min(x,1-x,y,1-y)*4),depth=Math.max(0,Math.min(1,(p.depth+1.3)/2.6));return hash(this.seed+':'+p.w.key)*.8+edge*.28+depth*.26+Math.min(.12,(p.w.neighbors?.length||0)*.012)-Math.min(1.4,(exposure.get(p.w.key)||0)/14000);}
 tick(now){const dt=this.lastTick===null?0:Math.max(0,Math.min(80,now-this.lastTick));this.lastTick=now;let moving=false,mid=0;
  for(const [key,r]of this.items){if(r.phase==='pending'){moving=true;}else if(r.phase==='enter'){r.opacity=Math.min(1,(now-r.fadeAt)/this.fade);if(r.opacity>=1)r.phase='visible';else moving=true;}else if(r.phase==='exit'){r.opacity=Math.max(0,r.exitFrom*(1-(now-r.fadeAt)/this.fade));if(r.opacity<=0){this.items.delete(key);this.stats.retired++;continue;}moving=true;}
   if(r.opacity>0){remember(key,dt*r.opacity);if(r.opacity<.98)mid++;}
  }this.stats.midFade=mid;return moving;
 }
 update(points,{now,width,height,zoom,blocked=[],placeRect,cap=13,selectedKey=null,hoverKey=null,advancing=false,motion=0,reduced=false,ready=()=>false,failed=()=>false}){
  const byKey=new Map(points.map(p=>[p.w.key,p])),locked=new Set([selectedKey,hoverKey].filter(Boolean));
  if(this.lastMotion!==null&&advancing)this.motionCredit+=Math.abs(motion-this.lastMotion);this.lastMotion=motion;
  for(const[key,r]of this.items){const p=byKey.get(key);if(p)r.p=p;if(locked.has(key)&&r.phase==='exit'){r.phase='visible';r.opacity=1;r.replace=null;}}
  // Wait for the exact new work's cache-only image before fading out its predecessor.
  for(const[key,r]of this.items)if(r.phase==='pending'){
   if(failed(r.p.w)){this.failed.add(key);this.items.delete(key);continue;}
   if(ready(r.p.w)){const victim=this.items.get(r.replace);if(victim&&locked.has(r.replace)){this.items.delete(key);continue;}if(victim){victim.phase='exit';victim.fadeAt=now;victim.exitFrom=victim.opacity;this.stats.handoffs++;}
    r.phase=reduced?'visible':'enter';r.fadeAt=now;r.opacity=reduced?1:0;r.born=now;this.stats.admitted++;this.seen.add(key);this.stats.distinct=this.seen.size;
   }
  }
  const cell=p=>Math.max(0,Math.min(3,Math.floor(p.x/width*4)))+4*Math.max(0,Math.min(2,Math.floor(p.y/height*3))),cells=new Map();
  const occupied=blocked.slice(),groups=new Set();for(const r of this.items.values()){cells.set(cell(r.p),(cells.get(cell(r.p))||0)+1);const rect={x:r.p.x+r.dx,y:r.p.y+r.dy,w:r.w,h:r.h};occupied.push(rect);groups.add(r.p.w.community||r.p.w.key);}
  const valid=points.filter(p=>p.x>24&&p.x<width-24&&p.y>85&&p.y<height-65&&!this.items.has(p.w.key)&&!this.failed.has(p.w.key));
  valid.sort((a,b)=>((cells.has(cell(b))?0:10)+this.score(b,width,height))-((cells.has(cell(a))?0:10)+this.score(a,width,height))||a.w.key.localeCompare(b.w.key));
  this.stats.candidates=valid.length;this.stats.projected=points.length;let pending=[...this.items.values()].some(r=>r.phase==='pending'||r.phase==='exit');
  const live=[...this.items.values()].filter(r=>r.phase!=='exit'),forced=selectedKey&&!this.items.has(selectedKey)?byKey.get(selectedKey):null;
  const initial=live.length<cap;
  if(!forced&&(pending||now-this.lastAdmission<(initial?140:this.interval)||(!initial&&(!advancing||this.motionCredit<.025))))return;
  let victim=null;
  if(!initial&&!forced){victim=live.filter(r=>!locked.has(r.p.w.key)&&r.phase==='visible'&&(now-r.born>=this.hold||((r.p.x<0||r.p.x>width||r.p.y<0||r.p.y>height)&&now-r.born>1200))).sort((a,b)=>(cells.get(cell(b.p))||0)-(cells.get(cell(a.p))||0)||this.score(a.p,width,height)-this.score(b.p,width,height))[0];if(!victim)return;}
  for(const p of forced?[forced]:valid){
   if(!forced&&groups.has(p.w.community||p.w.key))continue;
   if(victim&&(cells.get(cell(victim.p))||0)<=1&&cell(p)!==cell(victim.p)&&cells.has(cell(p)))continue;
   if(victim&&this.score(p,width,height)<this.score(victim.p,width,height)+.12&&cells.has(cell(p)))continue;
   const selected=p.w.key===selectedKey,w=selected?88:zoom>1.65?72:54,h=selected?98:zoom>1.65?80:60;
   const rect=placeRect(p,w,h,occupied,{x:14,y:82,w:width-28,h:height-152},12);if(!rect)continue;
   if(forced&&!initial)victim=live.filter(r=>!locked.has(r.p.w.key)&&r.phase==='visible').sort((a,b)=>this.score(a.p,width,height)-this.score(b.p,width,height))[0]||null;
   this.items.set(p.w.key,{p,dx:rect.x-p.x,dy:rect.y-p.y,w,h,phase:'pending',opacity:0,born:now,fadeAt:now,replace:victim?.p.w.key||null});this.lastAdmission=now;this.motionCredit=0;ready(p.w);break;
  }
 }
 snapshot(){return {...this.stats,count:this.items.size,visible:[...this.items.values()].filter(r=>r.opacity>.02).length,pending:[...this.items.values()].filter(r=>r.phase==='pending').length,entering:[...this.items.values()].filter(r=>r.phase==='enter').length,exiting:[...this.items.values()].filter(r=>r.phase==='exit').length};}
}
