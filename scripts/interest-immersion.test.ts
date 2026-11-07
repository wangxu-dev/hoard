import {expect,test} from 'bun:test';
import {readFileSync} from 'node:fs';
import {OrbitController} from '../web/src/pages/constellation-dev/orbit.js';
const read=(p:string)=>readFileSync(p,'utf8');
test('exploration fills the webview while ordinary library sidebar preferences stay intact',()=>{
 const side=read('web/src/components/app-sidebar.tsx'),app=read('web/src/App.tsx');
 expect(side).toContain('if (page === "explore") return');expect(side).toContain('explore-navigation');expect(side).toContain('aria-label="返回收藏库"');expect(side).toContain('onPageChange("settings")');expect(side).toContain('localStorage.getItem("hoard-sidebar")');expect(side).not.toContain('setOpen(true)');expect(app).toContain('music.queue && page !== "explore"');
});
test('clicking a work selects without changing any camera target; focus is an explicit action',()=>{
 const src=read('web/src/pages/constellation-dev/renderer.js'),body=src.slice(src.indexOf('function openWork('),src.indexOf('function back('));
 expect(body).toContain('selectedWork=w');expect(body).not.toContain('target=');expect(body).not.toContain('focusWork');expect(src).toContain('focusSelected:()=>{if(selectedWork)focusWork(selectedWork);}');expect(src).toContain('selected:false,paused:userPaused');
});
test('all restored view controls call live renderer operations, not placeholder controls',()=>{
 const page=read('web/src/pages/constellation-dev/index.tsx'),src=read('web/src/pages/constellation-dev/renderer.js');
 for(const action of ['back','reset','zoomIn','zoomOut','toggleRotation','focusSelected']){expect(page).toContain('controls'+(action==='toggleRotation'?'.toggleRotation':'?.'+action));expect(src).toContain(action);}
 expect(page).toContain('disabled={!panel?.work}');expect(page).toContain('aria-pressed={paused}');
});
test('manual pause stops rotation and resume starts smoothly, independent of selection',()=>{
 const o=new OrbitController(0);o.configure({paused:true},5000);expect(o.step(9000)).toBe(0);expect(o.phase).toBe('paused');o.configure({paused:false,selected:false},9000);expect(o.step(10000)).toBe(0);let turn=0;for(let t=10000;t<15000;t+=16)turn+=o.step(t);expect(turn).toBeGreaterThan(.05);
});
test('night palette and card overrides stay under exploration roots',()=>{
 const css=read('web/src/pages/constellation-dev/styles.css');expect(css).toContain('.explore-frame,#interest-native-root{');expect(css).toContain('#interest-native-root [data-testid=library-card]');expect(css).toContain('--popover:#14232e');expect(css).toContain('backdrop-filter:blur');expect(css).not.toMatch(/(?:^|\n)(?:html|body|:root|\.dark)\s*\{/);
});
