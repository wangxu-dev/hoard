import H from './core.js';
import {boundCamera} from './camera-bounds.js';
const clamp=(v,a,b)=>Math.max(a,Math.min(b,v));
// A single snapshot of real graph geometry, never a per-frame tracking target.
export function frameNeighborhood(work,byKey,camera,width,height,rotation=0){
 const neighbors=(work.neighbors||[]).map(n=>({work:byKey.get(n.key),weight:n.weight})).filter(n=>n.work&&n.weight>0).slice(0,36);
 const right=width<760?width*.43:Math.min(370,width*.3),rect={left:28,top:90,right:width-right-24,bottom:height-100};
 const points=[{work,weight:2},...neighbors],project=c=>points.map(n=>({...H.project(H.rotateZ(n.work.pos,rotation),c,width,height),weight:n.weight}));
 const inside=p=>p.x>=rect.left&&p.x<=rect.right&&p.y>=rect.top&&p.y<=rect.bottom;
 const before=project(camera),mass=before.reduce((s,p)=>s+p.weight,0),visible=before.reduce((s,p)=>s+(inside(p)?p.weight:0),0)/mass;
 const mostlyVisible=inside(before[0])&&visible>=.85;
 // No unrelated neighbors invented for isolates; only reveal them from behind UI.
 let zoom=camera.zoom;
 if(neighbors.length){
  if(mostlyVisible)zoom*=1.06;
  else {const xs=before.map(p=>p.x),ys=before.map(p=>p.y),fit=Math.min((rect.right-rect.left)*.8/Math.max(90,Math.max(...xs)-Math.min(...xs)),(rect.bottom-rect.top)*.8/Math.max(90,Math.max(...ys)-Math.min(...ys)));zoom*=clamp(fit,.92,1.24);}
 }
 const next=boundCamera({...camera,zoom},width,height),after=project(next);
 if(!mostlyVisible){
  const center=after.reduce((s,p)=>({x:s.x+p.x*p.weight/mass,y:s.y+p.y*p.weight/mass}),{x:0,y:0});
  next.panX+=clamp((rect.left+rect.right)/2-center.x,-width*.22,width*.22);
  next.panY+=clamp((rect.top+rect.bottom)/2-center.y,-height*.18,height*.18);
 }
 return {camera:boundCamera(next,width,height),neighborCount:neighbors.length,reason:neighbors.length?(mostlyVisible?'already-visible':'neighbor-bounds'):'isolated'};
}
// Restores the same explored world region without rewinding the rotating galaxy.
export function restoreExplorationView(view,rotation){
 const delta=rotation-(view.rotation||0),center=H.rotateZ({x:view.camera.tx,y:view.camera.ty,z:view.camera.tz},delta);
 return {...view.camera,yaw:view.camera.yaw-delta,tx:center.x,ty:center.y};
}
export class CameraFlight{
 constructor(duration=650){this.duration=duration;this.active=false;}
 start(from,to,now,reduced=false){this.from={...from};this.to={...to};this.started=now;this.active=!reduced;this.current=reduced?{...to}:{...from};return {...this.current};}
 cancel(){this.active=false;}
 step(now){if(!this.active)return {camera:{...this.current},moving:false};const t=clamp((now-this.started)/this.duration,0,1),ease=t*t*t*(10+t*(-15+6*t)),camera={...this.from};for(const k of Object.keys(this.to))camera[k]=this.from[k]+(this.to[k]-this.from[k])*ease;this.active=t<1;this.current=camera;return {camera,moving:this.active};}
}
