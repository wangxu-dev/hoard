import { graphLayout } from './graph-layout.js';
/* Independent geometric renderer. No external libraries or reference assets. */
'use strict';
const TAU=Math.PI*2;
function hash(s){let h=2166136261;for(let i=0;i<s.length;i++){h^=s.charCodeAt(i);h=Math.imul(h,16777619);}h^=h>>>16;h=Math.imul(h,0x85ebca6b);h^=h>>>13;h=Math.imul(h,0xc2b2ae35);return (h^(h>>>16))>>>0;}
function rnd(s){return hash(s)/4294967296;}
function normal(s){return Math.sqrt(-2*Math.log(Math.max(.00001,rnd(s+'a'))))*Math.cos(TAU*rnd(s+'b'));}
const spiralWinding=2.7;
function spiralAngle(radius,offset=0){return offset+radius*spiralWinding;}
function rotateZ(p,angle){const c=Math.cos(angle),s=Math.sin(angle);return {x:p.x*c-p.y*s,y:p.x*s+p.y*c,z:p.z};}
// Trailing arms: outward angular winding and material angular velocity have opposite signs.
function materialTurn(delta){return -Math.sign(spiralWinding)*delta;}
const palette=['#8edee5','#c6eee2','#a6d8f0','#c9e8dc','#81d9e1','#e2e9cd','#b5d3f1','#96e2ca','#e6d8b4','#96cde3','#d3e7e3'];
function prepare(data){
 const works=[],byKey=new Map(),pairs=new Map();
 for(const sc of data.scopes)for(const w of sc.works){
  const key=sc.source+':'+w.id;if(byKey.has(key))continue;
  const o={...w,key,source:sc.source,seed:rnd(key),neighbors:[]};works.push(o);byKey.set(key,o);
 }
 works.sort((a,b)=>a.key.localeCompare(b.key));
 for(const sc of data.scopes)for(const n of sc.neighbors||[]){
  const a=byKey.get(sc.source+':'+n.workId),b=byKey.get(sc.source+':'+n.to);
  if(!a||!b||a===b||!Number.isFinite(n.score)||n.score<=0)continue;
  const key=JSON.stringify([a.key,b.key].sort()),prior=pairs.get(key);
  if(!prior||n.score>prior.weight)pairs.set(key,{a,b,weight:n.score});
 }
 // Stable edge iteration also makes cold-cache/input-order results identical.
 const sortedPairs=new Map([...pairs].sort((a,b)=>a[0].localeCompare(b[0])));
 for(const {a,b,weight}of sortedPairs.values()){a.neighbors.push({key:b.key,weight});b.neighbors.push({key:a.key,weight});}
 for(const w of works)w.neighbors.sort((x,y)=>y.weight-x.weight||x.key.localeCompare(y.key));
 const layout=graphLayout(works,sortedPairs,{rnd,normal,hash,palette});
 return {works,byKey,...layout};
}
function sample(works,cap=6500,selectedKey=null){
 cap=Math.max(0,Math.floor(cap));if(!cap)return [];
 if(works.length<=cap)return works.slice();
 const rank=(a,b)=>hash(a.key)-hash(b.key)||a.key.localeCompare(b.key),linked=works.filter(w=>w.neighbors?.length),isolates=works.filter(w=>!w.neighbors?.length);
 // Keep visible pairs/community cores; purely random thinning was erasing actual structure.
 const groups=new Map();for(const w of linked){const c=w.community||w.key;if(!groups.has(c))groups.set(c,[]);groups.get(c).push(w);}
 const picked=[],used=new Set(),add=w=>{if(w&&!used.has(w.key)&&picked.length<cap){picked.push(w);used.add(w.key);}};
 if(selectedKey){const w=works.find(w=>w.key===selectedKey);add(w);const by=new Map(works.map(w=>[w.key,w]));for(const n of (w?.neighbors||[]).slice(0,120))add(by.get(n.key));}
 const linkedCap=Math.min(linked.length,Math.floor(cap*.7));
 for(const a of [...groups.values()].sort((a,b)=>a[0].community?.localeCompare(b[0].community)||rank(a[0],b[0]))){a.sort((x,y)=>y.neighbors.length-x.neighbors.length||rank(x,y));for(const w of a.slice(0,2)){if(picked.length>=linkedCap)break;add(w);}}
 for(const w of linked.sort(rank)){if(picked.length>=linkedCap)break;add(w);}
 for(const w of isolates.sort(rank))add(w);
 for(const w of works.slice().sort(rank))add(w);
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
 const next={...camera,zoom:Math.max(.55,Math.min(world?9:1.8,nextZoom))};
 if(world){const before=desired||project(world,camera,width,height),after=project(world,next,width,height);next.panX+=before.x-after.x;next.panY+=before.y-after.y;}
 return next;
}
function unprojectPlane(camera,x,y,width,height){const scale=Math.min(width*.375,height*.59)*camera.zoom,sx=(x-width*(camera.cx||.55)-camera.panX)/scale,sy=(y-height*.515-camera.panY)/scale,cp=Math.cos(camera.pitch),sp=Math.sin(camera.pitch),z=-camera.tz,ry=(3.6*sy-sy*z*cp+3.6*z*sp)/(3.6*cp+sy*sp),rx=sx*(3.6-ry*sp-z*cp)/3.6,ca=Math.cos(camera.yaw),sa=Math.sin(camera.yaw);return {x:rx*ca+ry*sa+camera.tx,y:-rx*sa+ry*ca+camera.ty,z:0};}
function wheelFactor(deltaY,deltaMode=0,ctrlKey=false){const pixels=deltaY*(deltaMode===1?16:deltaMode===2?600:1);return Math.exp(Math.max(-.65,Math.min(.65,-pixels*(ctrlKey?.004:.0015))));}
function cameraStep(camera,target,dt,reduced=false){const a=reduced?1:1-Math.exp(-10*Math.min(.05,dt)),next={...camera};let moving=false;for(const k of Object.keys(target)){const d=target[k]-camera[k];next[k]=Math.abs(d)<.00002?target[k]:camera[k]+d*a;if(Math.abs(d)>.00002)moving=true;}return {camera:next,moving};}
function inertiaStep(velocity,dt,decay=6.4){const t=Math.max(0,Math.min(.05,dt)),d=Math.exp(-decay*t);return {delta:velocity*(1-d)/decay,velocity:velocity*d};}
const api={spiralWinding,spiralAngle,rotateZ,materialTurn,hash,rnd,normal,prepare,sample,project,pick,mst,palette,overlapRect,placeRect,zoomAround,inertiaStep,wheelFactor,cameraStep,unprojectPlane};
export default api;
