import {expect,test} from 'bun:test';
import H from '../web/src/pages/constellation-dev/core.js';
import {OrbitController} from '../web/src/pages/constellation-dev/orbit.js';
const distance=(a:any,b:any)=>Math.hypot(a.x-b.x,a.y-b.y,a.z-b.z);
function fixture(){
 const works=Array.from({length:90},(_,i)=>({id:String(i)})),neighbors=[] as any[];
 for(let group=0;group<3;group++)for(let i=group*20;i<group*20+20;i++)for(let j=i+1;j<group*20+20;j++)neighbors.push({workId:String(i),to:String(j),score:.85});
 return {scopes:[{source:'douyin',works,neighbors}]};
}
test('actual graph cliques form separate, non-collapsed communities; isolates do not form a ring',()=>{
 const m=H.prepare(fixture());expect(m.stats.linked).toBe(60);expect(m.stats.isolated).toBe(30);expect(m.stats.components).toBe(3);
 const centers=[] as any[];let inside=0,count=0;
 for(let group=0;group<3;group++){const ws=m.works.filter((w:any)=>+w.id>=group*20&&+w.id<group*20+20),mean={x:0,y:0,z:0};for(const w of ws)for(const a of ['x','y','z'])mean[a]+=w.pos[a]/ws.length;centers.push(mean);for(const w of ws){inside+=distance(w.pos,mean);count++;}expect(new Set(ws.map((w:any)=>JSON.stringify(w.pos))).size).toBe(20);}
 const between=(distance(centers[0],centers[1])+distance(centers[1],centers[2])+distance(centers[2],centers[0]))/3;
 expect(inside/count).toBeLessThan(between*.2);
 const radii=m.works.filter((w:any)=>!w.neighbors.length).map((w:any)=>Math.hypot(w.pos.x,w.pos.y));expect(Math.min(...radii)).toBeLessThan(.55);expect(Math.max(...radii)).toBeGreaterThan(.9);
 const other=fixture();other.scopes[0].works.reverse();other.scopes[0].neighbors.reverse();const b=H.prepare(other);for(const w of m.works)expect(b.byKey.get(w.key).pos).toEqual(w.pos);
});
test('view packing ignores all preset topic/bucket labels and author fields',()=>{
 const f=fixture(),base=H.prepare(f);for(const w of f.scopes[0].works){Object.assign(w,{anchorTopicId:'fake',author:{name:'invented'}});}Object.assign(f.scopes[0],{topics:[{id:'fake',kind:'topic'}],topicMembership:[]});const m=H.prepare(f);for(const w of m.works)expect(w.pos).toEqual(base.byKey.get(w.key).pos);
});
test('selected work and its real neighbors survive display thinning',()=>{
 const m=H.prepare(fixture()),a=m.byKey.get('douyin:0'),picked=H.sample(m.works,35,a.key),keys=new Set(picked.map((w:any)=>w.key));expect(keys.has(a.key)).toBe(true);for(const n of a.neighbors)expect(keys.has(n.key)).toBe(true);
});
test('wheel delta modes agree; repeated wheel events accumulate and smooth without losing the cursor anchor',()=>{
 expect(H.wheelFactor(16,0)).toBeCloseTo(H.wheelFactor(1,1),10);
 const c={yaw:-.24,pitch:1.08,zoom:1,tx:0,ty:0,tz:0,panX:0,panY:0,cx:.5};let target={...c},camera={...c};const x=560,y=420,world=H.unprojectPlane(c,x,y,1100,800);
 for(let i=0;i<8;i++){const anchor=H.unprojectPlane(target,x,y,1100,800);target=H.zoomAround(target,anchor,target.zoom*H.wheelFactor(-60),1100,800,{x,y});camera=H.cameraStep(camera,target,1/60).camera;}
 expect(target.zoom).toBeGreaterThan(2);expect(camera.zoom).toBeLessThan(target.zoom);
 for(let i=0;i<100;i++){camera=H.cameraStep(camera,target,1/60).camera;const p=H.project(world,camera,1100,800);expect(Math.hypot(p.x-x,p.y-y)).toBeLessThan(.02);}
 expect(camera.zoom).toBeCloseTo(target.zoom,3);
});
test('quiet graph keeps rotating with frame-rate-independent material motion',()=>{
 const run=(fps:number)=>{const o=new OrbitController(0);let turn=0;for(let t=0;t<=12000;t+=1000/fps)turn+=H.materialTurn(o.step(t));return turn;};expect(run(60)).toBeLessThan(-.3);expect(Math.abs(run(60)-run(120))).toBeLessThan(.002);
});
