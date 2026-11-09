// Pure timeline/state helpers: no content IDs, graph changes, or offscreen spin.
const smooth=t=>{t=Math.max(0,Math.min(1,t));return t*t*(3-2*t);};
export function entranceLayers(elapsed,reduced=false){
 if(reduced)return {nebula:1,stars:1,covers:1,active:false};
 return {nebula:.36+.64*smooth(elapsed/420),stars:smooth((elapsed-40)/480),covers:smooth((elapsed-180)/480),active:elapsed<660};
}
let remembered=null;
export function rememberView(camera,rotation,paused){remembered={camera:{...camera},rotation,paused};}
export function recallView(){return remembered?{...remembered,camera:{...remembered.camera}}:null;}
export function landingCamera(camera){return {...camera,zoom:camera.zoom*1.035,pitch:Math.min(1.35,camera.pitch+.022)};}
