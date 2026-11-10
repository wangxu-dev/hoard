import {test,expect} from 'bun:test';
import H from '../web/src/pages/constellation-dev/core.js';
import {CoverDirector} from '../web/src/pages/constellation-dev/cover-director.js';
import {ImagePool} from '../web/src/pages/constellation-dev/image-pool.js';
const points=Array.from({length:480},(_,i)=>({x:65+(i%20)*54,y:110+Math.floor(i/20)*24,depth:(i%7)/7,w:{key:'fixture:'+i,community:'g'+i,neighbors:[],pos:{x:i,y:0,z:0}}}));
const options=(now:number,extra:any={})=>({now,width:1200,height:800,zoom:1.7,placeRect:H.placeRect,ready:()=>true,advancing:true,motion:now*.000055,...extra});
function step(d:any,now:number,extra:any={}){d.update(points,options(now,extra));d.tick(now);}
function filled(seed:string){const d=new CoverDirector({seed});for(let n=0;n<=6000;n+=100)step(d,n);return d;}
test('session seeds choose different real work IDs without touching spatial coordinates, spread over viewport',()=>{
 const a=filled('a'),b=filled('b'),keys=new Set(a.items.keys());expect([...b.items.keys()].filter(k=>keys.has(k)).length).toBeLessThan(8);expect(a.items.size).toBe(13);
 const cells=new Set([...a.items.values()].map(r=>Math.floor(r.p.x/1200*4)+4*Math.floor(r.p.y/800*3)));expect(cells.size).toBeGreaterThanOrEqual(10);
 for(const r of a.items.values()){const p=points.find(p=>p.w.key===r.p.w.key);expect(r.p).toBe(p);expect(p?.w.pos.x).toBe(Number(r.p.w.key.split(':')[1]));}
});
test('moving view changes real IDs one handoff at a time, with simultaneous partial opacity at distinct work positions',()=>{
 const d=filled('motion'),before=new Set(d.items.keys());let mid=false;
 for(let n=6100;n<=24000;n+=100){step(d,n);const r=[...d.items.values()];if(r.some(x=>x.phase==='exit'&&x.opacity>0&&x.opacity<1)&&r.some(x=>x.phase==='enter'&&x.opacity>0&&x.opacity<1))mid=true;expect(d.items.size).toBeLessThanOrEqual(14);}
 expect(d.stats.handoffs).toBeGreaterThan(2);expect([...d.items.keys()].some(k=>!before.has(k))).toBe(true);expect(mid).toBe(true);
});
test('pause keeps settled IDs stable, hover and selected cannot be retired while other real covers change',()=>{
 const d=filled('lock'),locked=[...d.items.keys()].slice(0,2);for(let n=6100;n<=20000;n+=100)step(d,n,{selectedKey:locked[0],hoverKey:locked[1]});for(const k of locked)expect(d.items.get(k)?.phase).not.toBe('exit');
 for(let n=20100;n<=22000;n+=100)step(d,n,{advancing:false,selectedKey:locked[0],hoverKey:locked[1]});const before=[...d.items.keys()].sort();for(let n=22100;n<=30000;n+=100)step(d,n,{advancing:false,selectedKey:locked[0],hoverKey:locked[1]});expect([...d.items.keys()].sort()).toEqual(before);
});
test('more than 128 unique cache reads remain possible with a strict bounded LRU and pinned active references',()=>{
 let disposed=0;const pool=new ImagePool(8,()=>disposed++);pool.set('pinned',{ready:true});pool.retain(['pinned']);for(let i=0;i<300;i++){expect(pool.set('u'+i,{ready:true})).toBe(true);expect(pool.size).toBeLessThanOrEqual(8);expect(pool.has('pinned')).toBe(true);}expect(pool.has('u299')).toBe(true);expect(pool.evictions).toBe(293);expect(disposed).toBe(293);pool.clear();expect(pool.size).toBe(0);
});
