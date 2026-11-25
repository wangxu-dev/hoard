import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {minimumImmersiveZoom,boundCamera} from '../web/src/pages/constellation-dev/camera-bounds.ts';
test('far zoom keeps edge coverage across aspect/pitch; every old snapshot is clamped',()=>{
 for(const[w,h]of[[1341,925],[1920,900],[800,1200],[920,640]])for(const pitch of[.2,.72,.94,1.35]){
  const floor=minimumImmersiveZoom(w,h,pitch);expect(floor).toBeGreaterThan(1.3);expect(floor).toBeLessThan(9);expect(boundCamera({pitch,zoom:.55},w,h).zoom).toBe(floor);expect(boundCamera({pitch,zoom:99},w,h).zoom).toBe(9);
  const scale=Math.min(w*.375,h*.59),r=1.4*.82*scale*floor;expect(Math.hypot(w*.5/r,h*.515/(r*Math.cos(pitch)))).toBeLessThanOrEqual(1);
 }
 expect(minimumImmersiveZoom(1341,925,1.35)).toBeGreaterThan(minimumImmersiveZoom(1341,925,.72));
});
test('wheel/buttons/reset/back/resize/tilt all use the same local camera boundary',()=>{
 const r=readFileSync('web/src/pages/constellation-dev/renderer.ts','utf8');expect(r).toContain('const next = boundCamera({ ...target, zoom: target.zoom * factor }, W, HH)');expect(r).toContain('flyTo(restoreExplorationView(v, worldRotation))');expect(r).toContain('flyTo({ ...overview })');expect(r).toMatch(/function flyTo\(next: CameraState\): void \{\s*stopInertia\(\);\s*target = boundCamera\(next, W, HH\)/);expect(r).toMatch(/function resize\(\): void \{\s*W = root\.clientWidth;\s*HH = root\.clientHeight;\s*camera = boundCamera\(camera, W, HH\);\s*target = boundCamera\(target, W, HH\)/);expect(r).toMatch(/camera = boundCamera\(camera, W, HH\);\s*target = \{ \.\.\.camera \};\s*invalidate\(\)/);
});
