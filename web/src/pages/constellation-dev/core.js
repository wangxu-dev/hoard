/* Independent geometric renderer. No external libraries or reference assets. */
'use strict';
const TAU=Math.PI*2;
function hash(s){let h=2166136261;for(let i=0;i<s.length;i++){h^=s.charCodeAt(i);h=Math.imul(h,16777619);}return h>>>0;}
function rnd(s){return hash(s)/4294967296;}
function normal(s){return Math.sqrt(-2*Math.log(Math.max(.00001,rnd(s+'a'))))*Math.cos(TAU*rnd(s+'b'));}
const spiralWinding=2.7;
function spiralAngle(radius,offset=0){return offset+radius*spiralWinding;}
function rotateZ(p,angle){const c=Math.cos(angle),s=Math.sin(angle);return {x:p.x*c-p.y*s,y:p.x*s+p.y*c,z:p.z};}
// Trailing arms: outward angular winding and material angular velocity have opposite signs.
function materialTurn(delta){return -Math.sign(spiralWinding)*delta;}
const palette=['#8edee5','#c6eee2','#a6d8f0','#c9e8dc','#81d9e1','#e2e9cd','#b5d3f1','#96e2ca','#e6d8b4','#96cde3','#d3e7e3'];
function prepare(data){
 const topics=new Map(),works=[],byKey=new Map(),pairs=new Map();
 for(const sc of data.scopes){
  for(const t of sc.topics){if(!topics.has(t.id))topics.set(t.id,{id:t.id,label:t.id==='t_no_text'?'无文案':t.label,kind:t.kind,all:[],color:'#6d8498'});}
  const ms=new Map();for(const m of sc.topicMembership){if(!ms.has(m.workId))ms.set(m.workId,[]);ms.get(m.workId).push(m);}
  for(const w of sc.works){
   const memberships=(ms.get(w.id)||[]).slice().sort((a,b)=>{
    const ea=a.evidence||[],eb=b.evidence||[];
    const length=x=>Math.max(0,...x.map(e=>(e.excerpt||'').length));
    return eb.length-ea.length||length(eb)-length(ea)||a.topicId.localeCompare(b.topicId);
   });
   const primary=w.anchorTopicId?memberships.find(m=>m.topicId===w.anchorTopicId):(data.schemaVersion>=2?null:memberships.find(m=>topics.get(m.topicId)?.kind==='topic')||memberships[0]);
   if(!primary)throw Error('Membership missing');
   const o={...w,key:sc.source+':'+w.id,source:sc.source,memberships,anchor:primary.topicId,seed:rnd(sc.source+':'+w.id)};
   works.push(o);byKey.set(o.key,o);topics.get(o.anchor).all.push(o);
  }
 }
 // Keep relations source-scoped and deduplicate reciprocal top-k neighbors.
 for(const sc of data.scopes)for(const n of sc.neighbors||[]){
  const a=byKey.get(sc.source+':'+n.workId),b=byKey.get(sc.source+':'+n.to);
  if(!a||!b||a===b||!Number.isFinite(n.score)||n.score<=0)continue;
  const key=JSON.stringify([a.key,b.key].sort()),prior=pairs.get(key);
  if(!prior||n.score>prior.weight)pairs.set(key,{a,b,weight:n.score});
 }
 // Flat scatter for every real work. No topic disks, no uncategorized outer ring.
 for(const t of topics.values()){t.center={x:0,y:0,z:0};t.angle=0;t.spread=.9;t.color=t.id==='t_no_text'?'#929aa6':palette[hash(t.id)%palette.length];}
 for(const w of works){
  const a=rnd(w.key+'arc')*TAU,r=Math.sqrt(rnd(w.key+'rad'))*.92;
  w.pos={x:Math.cos(a)*r,y:Math.sin(a)*r,z:normal(w.key+'dep')*.07};
  w.color=topics.get(w.anchor)?.color||'#6e8eaa';
  w.neighbors=[];
 }
 // Sparse weighted relaxation uses only real work pairs. It changes proximity,
 // never adds drawable points, theme rings, or ornamental relationship lines.
 const adjacency=new Map(),origins=new Map();
 for(const {a,b,weight}of pairs.values()){
  a.neighbors.push({key:b.key,weight});b.neighbors.push({key:a.key,weight});
  for(const [w,other]of [[a,b],[b,a]]){
   if(!adjacency.has(w.key)){adjacency.set(w.key,[]);origins.set(w.key,{...w.pos});}
   adjacency.get(w.key).push({other,weight});
  }
 }
 for(const w of works)w.neighbors.sort((x,y)=>y.weight-x.weight||x.key.localeCompare(y.key));
 for(let step=0;step<18;step++){
  const next=new Map();
  for(const [key,neighbors]of adjacency){
   const w=byKey.get(key),origin=origins.get(key),sum={x:0,y:0,z:0};let weight=0;
   for(const n of neighbors){weight+=n.weight;for(const axis of ['x','y','z'])sum[axis]+=n.other.pos[axis]*n.weight;}
   const pos={};for(const axis of ['x','y','z'])pos[axis]=w.pos[axis]*.45+sum[axis]/weight*.45+origin[axis]*.1;
   next.set(key,pos);
  }
  for(const [key,pos]of next)byKey.get(key).pos=pos;
 }
 return {topics,works,byKey};
}
function sample(works,cap=3000,selectedKey=null){
 cap=Math.max(0,Math.floor(cap));if(!cap)return [];
 if(works.length<=cap)return works.slice();
 const groups=new Map();for(const w of works){if(!groups.has(w.anchor))groups.set(w.anchor,[]);groups.get(w.anchor).push(w);}
 const sorted=[...groups.values()].map(a=>a.slice().sort((x,y)=>hash(x.key)-hash(y.key)||x.key.localeCompare(y.key))).sort((a,b)=>a[0].anchor.localeCompare(b[0].anchor));
 const picked=[],used=new Set();
 // Per-group floor prevents small, unknown and no-text populations vanishing.
 const floor=Math.min(24,Math.max(1,Math.floor(cap/sorted.length)));
 for(const a of sorted)for(const w of a.slice(0,Math.min(floor,a.length))){if(picked.length>=cap)break;picked.push(w);used.add(w.key);}
 const rest=works.filter(w=>!used.has(w.key)).sort((a,b)=>hash(a.key)-hash(b.key)||a.key.localeCompare(b.key));
 picked.push(...rest.slice(0,Math.max(0,cap-picked.length)));
 if(selectedKey&&!picked.some(w=>w.key===selectedKey)){const w=works.find(w=>w.key===selectedKey);if(w)picked[picked.length-1]=w;}
 return picked;
}
function transform(pos,camera){
 const x=pos.x-camera.tx,y=pos.y-camera.ty,z=pos.z-camera.tz;
 const ca=Math.cos(camera.yaw),sa=Math.sin(camera.yaw),cp=Math.cos(camera.pitch),sp=Math.sin(camera.pitch);
 const rx=x*ca-y*sa,ry=x*sa+y*ca;
 return {x:rx,y:ry*cp-z*sp,z:ry*sp+z*cp};
}
function project(pos,camera,width,height){
 const q=transform(pos,camera),p=3.6/Math.max(.65,3.6-q.z);
 const scale=Math.min(width*.375,height*.59)*camera.zoom;
 return {x:width*(camera.cx||.55)+q.x*scale*p+camera.panX,y:height*.515+q.y*scale*p+camera.panY,depth:q.z,p,scale};
}
function pick(buffer,x,y,max=10){let best=null,dist=max;for(const p of buffer){const d=Math.hypot(p.x-x,p.y-y);if(d<dist){dist=d;best=p;}}return best;}
function mst(stars,limit=38){
 // A sparse Euclidean spanning tree for visual organisation, NOT behavioural inference.
 const a=stars.slice().sort((x,y)=>hash(x.key)-hash(y.key)).slice(0,limit),edges=[];
 if(a.length<2)return edges;const used=new Set([0]);
 while(used.size<a.length){let best=null;for(const i of used)for(let j=0;j<a.length;j++)if(!used.has(j)){
  const p=a[i].pos,q=a[j].pos,d=(p.x-q.x)**2+(p.y-q.y)**2+(p.z-q.z)**2;
  if(!best||d<best.d)best={i,j,d};
 }if(!best)break;used.add(best.j);edges.push([a[best.i],a[best.j]]);}
 return edges;
}
function overlapRect(a,b,pad=0){return a.x<b.x+b.w+pad&&a.x+a.w+pad>b.x&&a.y<b.y+b.h+pad&&a.y+a.h+pad>b.y;}
function placeRect(point,width,height,occupied,bounds,gap=8){
 const offsets=[[15,-height-12],[15,16],[-width-16,-height-12],[-width-16,16],[18,-height-52],[35,48],[-width-35,48],[18,-height-94],[-width-18,-height-94]];
 for(const [dx,dy] of offsets){const r={x:point.x+dx,y:point.y+dy,w:width,h:height};if(r.x<bounds.x||r.y<bounds.y||r.x+r.w>bounds.x+bounds.w||r.y+r.h>bounds.y+bounds.h)continue;if(!occupied.some(o=>overlapRect(r,o,gap)))return r;}
 return null;
}
function zoomAround(camera,world,nextZoom,width,height,desired=null){
 const next={...camera,zoom:Math.max(.55,Math.min(world?5:1.8,nextZoom))};
 if(world){const before=desired||project(world,camera,width,height),after=project(world,next,width,height);next.panX+=before.x-after.x;next.panY+=before.y-after.y;}
 return next;
}
function inertiaStep(velocity,dt,decay=6.4){const t=Math.max(0,Math.min(.05,dt)),d=Math.exp(-decay*t);return {delta:velocity*(1-d)/decay,velocity:velocity*d};}
const api={spiralWinding,spiralAngle,rotateZ,materialTurn,hash,rnd,normal,prepare,sample,project,pick,mst,palette,overlapRect,placeRect,zoomAround,inertiaStep};
export default api;
