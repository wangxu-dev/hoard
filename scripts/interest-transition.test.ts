import { describe, expect, test } from "bun:test";
import { entranceLayers, rememberView, recallView, landingCamera } from "../web/src/pages/constellation-dev/entrance.js";
import { CameraFlight } from "../web/src/pages/constellation-dev/neighborhood-camera.js";

describe("exploration transitions", () => {
 test("nebula, real stars and covers enter in coordinated order", () => {
  expect(entranceLayers(0)).toEqual({nebula:.36,stars:0,covers:0,active:true});
  const early=entranceLayers(120); expect(early.nebula).toBeGreaterThan(early.stars);expect(early.covers).toBe(0);
  for(const key of ["nebula","stars","covers"] as const){let last=0;for(let t=0;t<=1000;t+=10){const a=entranceLayers(t)[key];expect(a).toBeGreaterThanOrEqual(last);expect(a).toBeLessThanOrEqual(1);last=a;}}
  expect(entranceLayers(1000)).toEqual({nebula:1,stars:1,covers:1,active:false});
 });
 test("reduced motion finishes immediately at every layer", () => {expect(entranceLayers(0,true)).toEqual({nebula:1,stars:1,covers:1,active:false});});
 test("return retains independent view and paused rotation without storing work IDs", () => {
  const c={yaw:.3,pitch:.8,zoom:2,panX:23,panY:-11,tx:.1,ty:.2,tz:0,cx:.5};
  rememberView(c,1.4,true);c.panX=900;const v=recallView()!;expect(v.camera.panX).toBe(23);expect(v.rotation).toBe(1.4);expect(v.paused).toBe(true);v.camera.yaw=99;expect(recallView()!.camera.yaw).toBe(.3);
 });
 test("landing is subtle and manual/new-neighbor flights take over continuously", () => {
  const c={yaw:.3,pitch:.8,zoom:2,panX:23,panY:-11,tx:.1,ty:.2,tz:0,cx:.5};const from=landingCamera(c);expect(from.zoom/c.zoom).toBeLessThan(1.05);expect(from.panX).toBe(c.panX);
  const flight=new CameraFlight();flight.start(from,c,0);const halfway=flight.step(250).camera;flight.cancel();expect(flight.active).toBe(false);const destination={...c,panX:35};expect(flight.start(halfway,destination,251)).toEqual(halfway);expect(flight.step(1000).camera).toEqual(destination);
 });
});
