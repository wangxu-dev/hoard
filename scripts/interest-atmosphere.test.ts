import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import H from '../web/src/pages/constellation-dev/core.js';
import {atmospherePatches} from '../web/src/pages/constellation-dev/atmosphere.js';
import {identityPlan} from '../web/src/pages/constellation-dev/identities.js';
import {detailAvatarURL} from '../web/src/pages/constellation-dev/avatar';
const work=(key:string,community:string)=>({key,community,seed:.3,neighbors:[{key:'other'}]});
test('nebula is projected background light, never a fake work or picking target',()=>{
 const p=atmospherePatches();expect(p.length).toBe(158);expect(p.every(x=>x.r>.05&&x.alpha>0&&Number.isFinite(x.x+x.y+x.z))).toBe(true);expect(p.every(x=>!('key'in x)&&!('neighbors'in x))).toBe(true);
 const r=readFileSync('web/src/pages/constellation-dev/renderer.js','utf8');expect(r).toContain('atmosphere.draw(ctx,project,W,HH)');expect(r).toContain('H.project(worldPoint(p),camera,W,HH)');
});
test('cover landmarks avoid details and each other; counts include every hidden candidate',()=>{
 const works=Array.from({length:40},(_,i)=>work('w'+i,'c'+i)),points=works.map((w,i)=>({w,x:90+(i%7)*100,y:140+Math.floor(i/7)*110,depth:0}));const blocked=[{x:720,y:0,w:280,h:800}];
 const plan=identityPlan(points,{selectedKey:'w0',width:1000,height:800,zoom:1,blocked,placeRect:H.placeRect,cap:9});expect(plan.length).toBeGreaterThan(3);expect(plan.length).toBeLessThanOrEqual(9);expect(plan[0].selected).toBe(true);
 const s=plan.stats;expect(s.projected).toBe(s.candidates+s.edgeFiltered);expect(s.candidates).toBe(s.drawn+s.capFiltered+s.collisionFiltered+s.communityFiltered);
 for(let i=0;i<plan.length;i++){expect(H.overlapRect(plan[i].rect,blocked[0],0)).toBe(false);for(let j=0;j<i;j++)expect(H.overlapRect(plan[i].rect,plan[j].rect,0)).toBe(false);}
});
test('only an opened author uses the normal native avatar resolver; external and cover URLs rejected',()=>{
 expect(detailAvatarURL('hoard-cover://localhost/a/1?cacheOnly=1')).toBe('hoard-cover://localhost/a/1');expect(detailAvatarURL('http://hoard-cover.localhost/a/1?cacheOnly=1')).toBe('http://hoard-cover.localhost/a/1');
 for(const u of['https://example.invalid/a/1','http://hoard-cover.localhost.evil/a/1','hoard-cover://localhost/w/1?cacheOnly=1','hoard-cover://evil/a/1','hoard-cover://localhost/a/1#x',''])expect(detailAvatarURL(u)).toBe('');
});
test('galaxy covers have no redundant work badge or author text; selected context stays visible',()=>{
 const r=readFileSync('web/src/pages/constellation-dev/renderer.js','utf8');expect(r).toContain('new ImagePool(128');expect(r).toContain('封面未缓存');expect(r).toContain('(related?1:.83)');expect(r).not.toContain('identity-name');expect(r).not.toContain('identity-kind');expect(r).not.toContain('author-placeholder');expect(r).not.toContain('fetch(');
 const css=readFileSync('web/src/pages/constellation-dev/styles.css','utf8');expect(css).toContain('.sky-identity.is-selected');
});

test('entry/reset share an immersive view, not a whole-disk zoom escape',()=>{const r=readFileSync('web/src/pages/constellation-dev/renderer.js','utf8');expect(r).toContain('const initial={yaw:-.24,pitch:.72,zoom:1.7');expect(r).toContain('const overview={...initial}');expect(r).toContain('target=boundCamera({...overview},W,HH)');expect(r).toContain('target=boundCamera({...v.camera},W,HH)');});
