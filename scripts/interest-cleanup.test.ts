import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { OrbitController } from "../web/src/pages/constellation-dev/orbit.js";

function advance(controller: OrbitController, from: number, to: number, fps=60) {
  let angle = 0, now=from;
  while(now < to) { now=Math.min(to,now+1000/fps); angle+=controller.step(now); }
  return angle;
}
describe("Explore clean view and single-clock motion", () => {
 test("three seconds quiet, then smooth bounded resumption", () => {
  const o=new OrbitController(0);
  expect(advance(o,0,3000)).toBe(0);expect(o.phase).toBe("waiting");
  const first=advance(o,3000,3100);expect(first).toBeGreaterThan(0);expect(first).toBeLessThan(.055*.1);
  const ramp=first+advance(o,3100,4000);expect(ramp).toBeCloseTo(.055*.5,10);
  expect(advance(o,4000,5000)).toBeCloseTo(.055,10);expect(o.phase).toBe("rotating");
 });
 test("theme or work selection holds indefinitely and cancellation waits anew", () => {
  const o=new OrbitController(0);advance(o,0,5000);o.configure({selected:true},5000);
  expect(advance(o,5000,15000)).toBe(0);expect(o.phase).toBe("selected");
  o.configure({selected:false},15000);expect(advance(o,15000,18000)).toBe(0);
  expect(advance(o,18000,19000)).toBeCloseTo(.055*.5,10);
 });
 test("hover exit and drag release each restore after three seconds without a focus latch", () => {
  for (const flag of ["hover","drag"]) {
   const o=new OrbitController(0);advance(o,0,5000);o.configure({[flag]:true},5000);
   expect(advance(o,5000,7000)).toBe(0);expect(o.canTick()).toBe(false);
   o.configure({[flag]:false},7000);expect(o.canTick()).toBe(true);
   expect(advance(o,7000,10000)).toBe(0);expect(advance(o,10000,11000)).toBeCloseTo(.0275,10);
  }
 });
 test("busy camera and reduced-motion never rotate; preference exit starts fresh", () => {
  const o=new OrbitController(0,true);expect(advance(o,0,10000)).toBe(0);
  o.configure({reduced:false,busy:true},10000);expect(advance(o,10000,12000)).toBe(0);
  o.configure({busy:false},12000);expect(advance(o,12000,15000)).toBe(0);expect(advance(o,15000,16000)).toBeCloseTo(.0275,10);
 });
 test("repeated pauses/cancellations cannot accelerate or stack callbacks", () => {
  const o=new OrbitController(0);let now=0;
  for(let i=0;i<50;i++){o.configure({selected:true},now);o.configure({selected:false},now+10);advance(o,now+10,now+5000);now+=5000;}
  expect(advance(o,now,now+1000)).toBeCloseTo(.055,10);
  const source=readFileSync("web/src/pages/constellation-dev/orbit.js","utf8");expect(source).not.toMatch(/setTimeout|setInterval|document\.activeElement|flags.*focus/);
 });
 test("frame-rate independent yaw and no huge hidden-tab catch-up", () => {
  const angles=[30,60,120].map(fps=>advance(new OrbitController(0),0,6000,fps));expect(angles[0]).toBeCloseTo(angles[1],10);expect(angles[1]).toBeCloseTo(angles[2],10);
  const o=new OrbitController(0);advance(o,0,5000);o.configure({hidden:true},5000);o.configure({hidden:false},300000);expect(o.step(300000)).toBe(0);expect(advance(o,300000,303000)).toBe(0);
 });
 test("a stale animation timestamp cannot rewind the activity clock",()=>{
  const o=new OrbitController(0);o.activity(1000);expect(o.step(990)).toBe(0);expect(o.lastStep).toBe(1000);expect(advance(o,1000,4000)).toBe(0);expect(advance(o,4000,5000)).toBeCloseTo(.0275,10);
 });
 test("deleted controls/explanations are absent, not hidden in a tooltip", () => {
  const shell=readFileSync("web/src/pages/constellation-dev/shell.html","utf8");
  expect(shell).not.toMatch(/<header|<footer|<button|<input|<details|<summary|view-tools|id="(?:sources|search|index|trail|mode|baseline|count)"/);
  const visible=["index.tsx","renderer.js","details.tsx","shell.html"].map(f=>readFileSync(`web/src/pages/constellation-dev/${f}`,"utf8")).join("\n");
  expect(visible).not.toMatch(/关于这条收藏|关于这个星群|主归属|画布显示|归类依据|置信|抽样|开发预览|字面线索|误检|真实收藏时间|原生一致快照|未归类|无文案作品|[▶▼×↺▷Ⅱ⌕☷]/);
  const details=readFileSync("web/src/pages/constellation-dev/details.tsx","utf8");expect(details).toContain('from "lucide-react"');expect(details).toContain('<Button variant="ghost" size="icon"');expect(details).not.toContain("innerHTML"); expect(details).not.toContain("再看一些");
 });
 test("exploration is normal navigation in both development and release", () => {
  const side=readFileSync("web/src/components/app-sidebar.tsx","utf8"), app=readFileSync("web/src/App.tsx","utf8");
  expect(side).toContain('if (page === "explore") { setOpen(true);');expect(side).not.toContain('setOpen(false)');expect(side).toContain('localStorage.getItem("hoard-sidebar")');expect(side).toContain('{ id: "explore" as const, icon: Compass }');expect(app).toContain('const ExplorePage = lazy(');expect(app).toContain('useState<Page>("library")');expect(app).toContain('page === "explore" ?');expect(side+app).not.toMatch(/interestDev|import\.meta\.env\.DEV|URLSearchParams/);
 });
});
