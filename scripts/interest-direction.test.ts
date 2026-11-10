import { expect,test } from "bun:test";
import { readFileSync } from "node:fs";
import H from "../web/src/pages/constellation-dev/core.js";
const camera={yaw:-.24,pitch:.86,zoom:1,tx:0,ty:0,tz:0,panX:0,panY:0,cx:.53};
function screenHandedness(pitch:number){const c={...camera,pitch},o=H.project({x:0,y:0,z:0},c,1000,800),x=H.project({x:.1,y:0,z:0},c,1000,800),y=H.project({x:0,y:.1,z:0},c,1000,800);return (x.x-o.x)*(y.y-o.y)-(x.y-o.y)*(y.x-o.x);}
test("positive outward winding requires negative material rotation for trailing arms",()=>{const winding=(H.spiralAngle(.6)-H.spiralAngle(.3))/.3;expect(winding).toBeGreaterThan(0);expect(winding*H.materialTurn(.055)).toBeLessThan(0);});
test("default projection maps positive angles clockwise in downward screen-y; reverse side flips",()=>{expect(screenHandedness(.86)).toBeGreaterThan(0);expect(screenHandedness(Math.PI-.86)).toBeLessThan(0);});
test("auto material rotation does not reverse manual camera dragging",()=>{const source=readFileSync("web/src/pages/constellation-dev/renderer.js","utf8");expect(source).toContain("worldRotation+=H.materialTurn(delta)");expect(source).toContain("camera.yaw-=dx*.005");expect(source).not.toContain("camera.yaw+=delta");});
test("projection/picking and cursor zoom use the same material transform",()=>{const p={x:.5,y:.3,z:.02},rotated=H.rotateZ(p,-.8),before=H.project(rotated,camera,1000,800),next=H.zoomAround(camera,rotated,4.5,1000,800),after=H.project(rotated,next,1000,800);expect(after.x).toBeCloseTo(before.x,8);expect(after.y).toBeCloseTo(before.y,8);expect(p).toEqual({x:.5,y:.3,z:.02});const picked=H.pick([{...before,w:{key:"fixture"}}],before.x,before.y,5);expect(picked.w.key).toBe("fixture");});
