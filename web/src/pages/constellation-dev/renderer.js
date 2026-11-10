/* Independent image-led local prototype. Content metadata is never inferred by the renderer. */
import { matchesPublication } from './publication-time.ts';
import H from './core.js';
import {entranceLayers,rememberView,recallView,landingCamera} from './entrance.js';
import {CameraFlight,frameNeighborhood,restoreExplorationView} from './neighborhood-camera.js';
import {RelationLayer} from './relation-lines.js';
import { OrbitController } from './orbit.js';
import { createAtmosphere } from './atmosphere.js';
import { CoverDirector } from './cover-director.js';
import { ImagePool } from './image-pool.js';
import { minimumImmersiveZoom, boundCamera } from './camera-bounds.js';
export function mountInterestSky(root,D,options={}){
const M=H.prepare(D),atmosphere=createAtmosphere(),flight=new CameraFlight(),relations=new RelationLayer(M.works,M.byKey);
let publicationSelection={range:null,includeUnknown:false};
let matchingKeys=new Set(M.works.map(w=>w.key));
let renderedKeys=new Set(),relationStats={},focusWeights=new Map();
console.warn("[interest-dev] graph",JSON.stringify({analysisVersion:D.analysisVersion,total:M.works.length,linked:M.works.filter(w=>w.neighbors.length).length,edges:M.works.reduce((s,w)=>s+w.neighbors.length,0)/2}));
const layoutFingerprint=M.works.reduce((v,w)=>Math.imul(v^H.hash(w.key+':'+[w.pos.x,w.pos.y,w.pos.z].map(x=>x.toFixed(6)).join(',')),16777619)>>>0,2166136261);
const listeners=[];let disposed=false;
const on=(target,type,handler,options)=>{target.addEventListener(type,handler,options);listeners.push(()=>target.removeEventListener(type,handler,options));};
const $=s=>root.querySelector(s),make=(t,c,text)=>{const e=document.createElement(t);if(c)e.className=c;if(text!==undefined)e.textContent=text;return e;},srcName=s=>s==='bilibili'?'B站':'抖音';
const canvas=$('#sky'),ctx=canvas.getContext('2d',{alpha:false}),reducedQuery=matchMedia('(prefers-reduced-motion: reduce)');let reduced=reducedQuery.matches;const orbitTokens=getComputedStyle(document.documentElement);const orbit=new OrbitController(performance.now(),reduced,{delay:parseFloat(orbitTokens.getPropertyValue('--explore-orbit-idle')),ramp:parseFloat(orbitTokens.getPropertyValue('--explore-orbit-ramp'))});
let W=root.clientWidth,HH=root.clientHeight,dpr=Math.min(devicePixelRatio||1,1.5),source='all';
const initial={yaw:-.24,pitch:.72,zoom:1.7,tx:0,ty:0,tz:0,panX:0,panY:0,cx:.5};
const overview={...initial};
const savedView=recallView(),enteredAt=performance.now();let layers=entranceLayers(0,reduced);
let worldRotation=savedView?.rotation||0,lastMotionPhase="",userPaused=savedView?.paused||false;
let camera=boundCamera({...(savedView?.camera||initial)},W,HH),target={...camera},active=[],stars=[],buffer=[],avatarBuffer=[],selectedWork=null,history=[],hover=null,mouse={x:-1000,y:-1000},drag=null,inertia={yaw:0,pitch:0,panX:0,panY:0},frame=0,last=0,dirty=true,hoverId='';
if(import.meta.env.DEV)console.debug("[interest-dev] page-view",JSON.stringify({phase:"restore",remembered:!!savedView,camera:target,rotation:worldRotation,paused:userPaused}));
camera=flight.start(boundCamera(landingCamera(target),W,HH),target,enteredAt,reduced);
const imageStates=new Map();let diagnosticTimer=0;
const frameSamples=[],frameGaps=[];let identityDrawn=0,identityCovers=0,identityMissing=0,identityLast=0,identityStats={};
const identityCards=new Map(),identityRoot=root.querySelector("#sky-identities"),director=new CoverDirector();
function imageCounts(){const a=[...imageStates],kind=k=>{const rows=a.filter(([url])=>url.includes('/'+k+'/'));return {requested:rows.length,loaded:rows.filter(([,s])=>s===1).length,failed:rows.filter(([,s])=>s===-1).length};};return {requested:a.length,loaded:a.filter(([,s])=>s===1).length,failed:a.filter(([,s])=>s===-1).length,covers:kind('w'),avatars:kind('a')};}
function recordImage(url,state){imageStates.set(url,state);clearTimeout(diagnosticTimer);diagnosticTimer=setTimeout(()=>console.warn("[interest-dev] assets",JSON.stringify(imageCounts())),250);}
const imageCache=new ImagePool(128,r=>{r.img.onload=null;r.img.onerror=null;r.img.removeAttribute('src');}),glows=new Map(),starFade=new Map();
const asset=v=>v?(D.assets?.[v]||v):null,cover=w=>asset(w?.coverRef||w?.coverURL||w?.cover),author=w=>typeof w?.author==='object'?w.author:null,avatar=w=>asset(author(w)?.avatarRef||author(w)?.avatarURL||w?.authorAvatar),authorName=w=>author(w)?.name||(typeof w?.author==='string'?w.author:'');
function image(ref){const url=asset(ref);if(!url)return null;let rec=imageCache.get(url);if(!rec){const im=new Image();im.decoding='async';im.referrerPolicy='no-referrer';rec={img:im,ready:false,failed:false};if(!imageCache.set(url,rec))return null;recordImage(url,0);im.onload=()=>{rec.ready=true;recordImage(url,1);invalidate();};im.onerror=()=>{rec.failed=true;recordImage(url,-1);invalidate();};im.src=url;}return rec.ready?rec.img:null;}
function attachImage(e,url){if(!url){e.hidden=true;e.removeAttribute('src');return false;}e.hidden=false;e.referrerPolicy='no-referrer';recordImage(url,0);e.onload=()=>{recordImage(url,1);e.classList.add('ready');};e.onerror=()=>{recordImage(url,-1);e.hidden=true;e.classList.remove('ready');if(e.id==='preview-cover')$('#preview').classList.add('no-art');};e.classList.remove('ready');e.src=url;return true;}
function glow(color){if(glows.has(color))return glows.get(color);const c=document.createElement('canvas');c.width=c.height=64;const x=c.getContext('2d'),r=parseInt(color.slice(1,3),16),g=parseInt(color.slice(3,5),16),b=parseInt(color.slice(5,7),16),v=x.createRadialGradient(32,32,0,32,32,32);v.addColorStop(0,`rgba(${r},${g},${b},.75)`);v.addColorStop(.12,`rgba(${r},${g},${b},.32)`);v.addColorStop(.45,`rgba(${r},${g},${b},.075)`);v.addColorStop(1,`rgba(${r},${g},${b},0)`);x.fillStyle=v;x.fillRect(0,0,64,64);glows.set(color,c);return c;}
const worldPoint=p=>H.rotateZ(p,worldRotation);
const project=p=>H.project(worldPoint(p),camera,W,HH);
function invalidate(){if(disposed)return;dirty=true;if(!frame&&!document.hidden)frame=requestAnimationFrame(draw);}
function resize(){W=root.clientWidth;HH=root.clientHeight;camera=boundCamera(camera,W,HH);target=boundCamera(target,W,HH);dpr=Math.min(devicePixelRatio||1,1.5);canvas.width=Math.round(W*dpr);canvas.height=Math.round(HH*dpr);ctx.setTransform(dpr,0,0,dpr,0,0);invalidate();}
function allowed(w){return source==='all'||w.source===source;}
function timeMatches(w){return matchingKeys.has(w.key);}
function sampleStars(){
 const matching=active.filter(timeMatches),context=active.filter(w=>!timeMatches(w));
 const primary=H.sample(matching,6500,selectedWork?.key);
 return primary.concat(H.sample(context,Math.max(0,6500-primary.length)));
}
function setPublicationRange(selection){
 publicationSelection=selection;matchingKeys=new Set(M.works.filter(w=>matchesPublication(w,selection)).map(w=>w.key));
 stars=sampleStars();renderedKeys=new Set(stars.map(w=>w.key));
 for(const w of stars)if(!starFade.has(w.key))starFade.set(w.key,reduced?1:0);
 for(const key of starFade.keys())if(!renderedKeys.has(key))starFade.delete(key);
 const now=performance.now();
 for(const [key,r] of director.items)if(!matchingKeys.has(key)){if(r.phase==='pending'){director.items.delete(key);}else if(r.phase!=='exit'){r.phase='exit';r.fadeAt=now;r.exitFrom=r.opacity;}}
 director.lastAdmission=-Infinity;identityLast=0;hidePreview();renderPanel();invalidate();
}
function cancelFlight(){if(flight.active){flight.cancel();target={...camera};}}
function flyTo(next){stopInertia();target=boundCamera(next,W,HH);camera=flight.start(camera,target,performance.now(),reduced);syncMotion(performance.now(),false);orbit.resumeNatural(performance.now());invalidate();}
function stopInertia(){inertia={yaw:0,pitch:0,panX:0,panY:0};}
function interacted(){orbit.activity(performance.now());}
function syncMotion(now=performance.now(),busy=false){orbit.configure({hover:!!hover,drag:!!drag,selected:false,paused:userPaused,hidden:document.hidden,reduced,busy},now);}
function refresh(){active=M.works.filter(allowed);stars=sampleStars();const keys=new Set(stars.map(w=>w.key));renderedKeys=keys;for(const k of starFade.keys())if(!keys.has(k))starFade.delete(k);for(const w of stars)if(!starFade.has(w.key))starFade.set(w.key,reduced?1:0);if(selectedWork&&!allowed(selectedWork))selectedWork=null;hidePreview();renderPanel();invalidate();}
function snapshot(){return {camera:{...camera},work:selectedWork?.key||null,rotation:worldRotation};}
function pushView(){history.push(snapshot());if(history.length>12)history.shift();}
function focusWork(w){hidePreview();const plan=frameNeighborhood(w,M.byKey,camera,W,HH,worldRotation);flyTo(plan.camera);$('#live').textContent=plan.neighborCount?'正在查看真实词语邻居':'此作品暂未找到词语邻居';}
function openWork(w,push=true){if(!allowed(w))return;if(push)pushView();selectedWork=w;stars=sampleStars();renderedKeys=new Set(stars.map(w=>w.key));hidePreview();renderPanel();focusWork(w);$('#live').textContent=(w.title||'未命名收藏')+(w.neighbors.length?'':'，暂未找到词语邻居');}
function back(){const v=history.pop();hidePreview();if(v){selectedWork=v.work?M.byKey.get(v.work):null;stars=sampleStars();renderedKeys=new Set(stars.map(w=>w.key));renderPanel();flyTo(restoreExplorationView(v,worldRotation));}else reset();}
function reset(){history=[];selectedWork=null;hidePreview();stars=sampleStars();renderedKeys=new Set(stars.map(w=>w.key));renderPanel();flyTo({...overview});}
function contentWork(w){return {publishedAt:w.publishedAt||0,outsideTimeRange:!timeMatches(w),key:w.key,id:w.id,sourceCode:w.source,kind:w.mediaKind==='image'?'gallery':'video',title:w.title||'未命名收藏',source:srcName(w.source),author:authorName(w),avatar:avatar(w)||'',cover:cover(w)||'',topics:[]};}
function renderPanel(){
 syncMotion();
 if(!selectedWork){options.onPanel?.(null);return;}
 const neigh=(selectedWork.neighbors||[]).map(n=>M.byKey.get(n.key)).filter(Boolean);
 const ws=neigh.filter(w=>w.key!==selectedWork.key);
 options.onPanel?.({key:D.snapshotId+':'+source+':work:'+selectedWork.key,label:neigh.length?'相关作品':'作品',work:contentWork(selectedWork),works:ws.map(contentWork),back,openWork:key=>{const w=M.byKey.get(key);if(w)openWork(w);},focusTheme:()=>{},imageStatus:recordImage});
}
function hidePreview(){hover=null;hoverId='';$('#preview').classList.remove('visible');$('#hover-group').hidden=true;syncMotion();invalidate();}
function showPreview(w){
 if(hoverId!==w.key){hoverId=w.key;const c=cover(w),card=$('#preview');card.classList.toggle('no-art',!c);attachImage($('#preview-cover'),c);attachImage($('#preview-avatar'),avatar(w));$('#preview-author').textContent=authorName(w)||srcName(w.source);$('#preview-title').textContent=w.title||'未命名收藏';$('#preview-theme').textContent='';$('#preview-kind').textContent=w.mediaKind==='image'?'图文':'视频';}
 const card=$('#preview'),height=cover(w)?303:137;card.style.left=Math.max(14,Math.min(W-260,mouse.x+(mouse.x>W*.62?-266:25)))+'px';card.style.top=Math.max(16,Math.min(HH-height-16,mouse.y-60))+'px';card.classList.add('visible');$('#hover-group').hidden=true;
}
function pickAt(x,y){let best=null,dist=Infinity;for(const p of avatarBuffer){const v=Math.hypot(p.x-x,p.y-y);if(v<p.r+4&&v<dist){dist=v;best=p;}}return best||H.pick(buffer.filter(p=>timeMatches(p.w)),x,y,11);}
function hoverAt(e){interacted();const bounds=root.getBoundingClientRect();mouse={x:e.clientX-bounds.left,y:e.clientY-bounds.top};if(drag)return;const p=pickAt(mouse.x,mouse.y);hover=p;if(p){stopInertia();canvas.style.cursor='pointer';showPreview(p.w);}else{
 $('#preview').classList.remove('visible');hoverId='';$('#hover-group').hidden=true;canvas.style.cursor='grab';}
 syncMotion();invalidate();
}
function relatedStar(w){return !!selectedWork&&(w.key===selectedWork.key||(selectedWork.neighbors||[]).some(n=>n.key===w.key));}
function canOrbit(){return orbit.canTick();}
function updateCamera(dt,now){let moving=false;const navigating=flight.active;
 if(!drag&&!reduced){for(const k of ['yaw','pitch','panX','panY']){const stop=k.startsWith('pan')?.4:.004;if(Math.abs(inertia[k])>stop){const v=H.inertiaStep(inertia[k],dt);camera[k]+=v.delta;inertia[k]=v.velocity;if(k==='pitch'){const c=Math.max(.2,Math.min(1.35,camera.pitch));if(c!==camera.pitch)inertia[k]=0;camera.pitch=c;}target[k]=camera[k];moving=true;}else inertia[k]=0;}}
 camera=boundCamera(camera,W,HH);target=boundCamera(target,W,HH);
 if(!drag){if(navigating){const step=flight.step(now);camera=boundCamera(step.camera,W,HH);moving=step.moving;if(!flight.active)target={...camera};}else{const step=H.cameraStep(camera,target,dt,reduced);camera=step.camera;moving=step.moving||moving;}}
 syncMotion(now,moving&&!navigating);const delta=orbit.step(now);if(delta){worldRotation+=H.materialTurn(delta);moving=true;}if(orbit.phase!==lastMotionPhase){lastMotionPhase=orbit.phase;console.warn("[interest-dev] motion",JSON.stringify({phase:orbit.phase,worldRotation,cameraYaw:camera.yaw,cameraPitch:camera.pitch}));}return moving;
}
function draw(now){
 if(disposed)return;const began=performance.now();if(last&&now-last<250){frameGaps.push(now-last);if(frameGaps.length>240)frameGaps.shift();}frame=0;const dt=Math.max(.001,Math.min(.05,(now-last)/1000||.016));last=now;layers=entranceLayers(now-enteredAt,reduced);let anim=updateCamera(dt,now)||layers.active;if(!dirty&&!anim){if(canOrbit())frame=requestAnimationFrame(draw);return;}dirty=false;ctx.setTransform(dpr,0,0,dpr,0,0);
 ctx.fillStyle='#0b131d';ctx.fillRect(0,0,W,HH);const b=ctx.createRadialGradient(W*.53,HH*.49,10,W*.53,HH*.49,W*.63);b.addColorStop(0,'#11232f');b.addColorStop(.47,'#0d1b27');b.addColorStop(1,'#0a121d');ctx.fillStyle=b;ctx.fillRect(0,0,W,HH);
 atmosphere.draw(ctx,project,W,HH,layers.nebula);
 const relationFrame=relations.frame(selectedWork,hover?.w);focusWeights=relationFrame.weights;relationStats=relations.draw(ctx,relationFrame,project,renderedKeys,W,HH,layers.stars,timeMatches);
 const response=new Set(hover?(hover.w.neighbors||[]).map(n=>n.key):[]); if(hover)response.add(hover.w.key);
 buffer=stars.map(w=>({...project(w.pos),w})).filter(p=>p.x>-20&&p.x<W+20&&p.y>0&&p.y<HH).sort((a,b)=>a.depth-b.depth);
 for(const p of buffer){const w=p.w,related=!selectedWork||focusWeights.has(w.key),bright=response.size&&response.has(w.key),same=hover&&author(w)?.id&&author(w).id===author(hover.w)?.id;
  let f=starFade.get(w.key)??1;if(f<.997){f=reduced?1:f+(1-f)*Math.min(1,dt*8);starFade.set(w.key,f);anim=true;}
  const linked=!!w.neighbors.length;
  const depthFade=Math.max(.46,Math.min(1.13,(p.depth+1.7)/2.3));
  const alpha=layers.stars*(timeMatches(w)?1:.12)*depthFade*(linked?.82:.30)*(related?1:.83)*(response.size&&!bright&&!same?.86:1)*f;
  const r=Math.min(3,Math.max(.45,(linked?.75+w.seed*.85:.45+w.seed*.5)*p.p*camera.zoom**.16));ctx.globalAlpha=alpha;ctx.fillStyle=linked?(w.seed>.6?'#f0ede0':w.color):w.color;ctx.beginPath();ctx.arc(p.x,p.y,r,0,Math.PI*2);ctx.fill();
  if((linked&&w.seed>.35)||bright||same){const rr=Math.min(18,(4+r*2.4)*(bright||same?1.25:1));ctx.globalAlpha=(bright||same?.68:.30)*alpha;ctx.globalCompositeOperation='screen';ctx.drawImage(glow(bright?'#b5f4ef':w.color),p.x-rr,p.y-rr,rr*2,rr*2);ctx.globalCompositeOperation='source-over';}
  if(w.key===selectedWork?.key&&timeMatches(w)){ctx.globalAlpha=layers.stars*.75*depthFade;ctx.globalCompositeOperation='screen';ctx.drawImage(glow('#f5b298'),p.x-30,p.y-30,60,60);ctx.globalCompositeOperation='source-over';ctx.globalAlpha=layers.stars*Math.min(1,depthFade);ctx.strokeStyle='#f5ae94';ctx.lineWidth=1.6;ctx.beginPath();ctx.arc(p.x,p.y,14,0,Math.PI*2);ctx.stroke();ctx.globalAlpha=layers.stars*.6*depthFade;ctx.beginPath();ctx.arc(p.x,p.y,19,0,Math.PI*2);ctx.stroke();}
  else if(w.key===hover?.w.key){ctx.globalAlpha=layers.stars*depthFade;ctx.strokeStyle='#c8fbf2';ctx.lineWidth=1;ctx.beginPath();ctx.arc(p.x,p.y,10,0,Math.PI*2);ctx.stroke();}
  else if(focusWeights.has(w.key)&&timeMatches(w)){const weight=Math.sqrt(focusWeights.get(w.key));ctx.globalAlpha=layers.stars*(.22+.55*weight)*depthFade;ctx.strokeStyle='#a6e5db';ctx.lineWidth=.55+weight;ctx.beginPath();ctx.arc(p.x,p.y,4+3*weight,0,Math.PI*2);ctx.stroke();}
 }
 ctx.globalAlpha=1;avatarBuffer=[];
 if(now-identityLast>320){renderIdentities(now);identityLast=now;}if(director.tick(now))anim=true;updateIdentityVisuals();
 const vig=ctx.createRadialGradient(W*.52,HH*.49,HH*.28,W*.52,HH*.49,W*.7);vig.addColorStop(0,'#08121b00');vig.addColorStop(1,'#08121b55');ctx.fillStyle=vig;ctx.globalAlpha=1;ctx.fillRect(0,0,W,HH);
 frameSamples.push(performance.now()-began);if(frameSamples.length>240)frameSamples.shift();
 if(anim||canOrbit()){dirty=anim;frame=requestAnimationFrame(draw);}
}
function renderIdentities(now){
 const occupied=[];const rootRect=root.getBoundingClientRect();
 for(const selector of ['.explore-navigation','.explore-controls','.explore-timeline','[data-explore-details]','[data-testid=music-bar]'])for(const e of document.querySelectorAll(selector)){const r=e.getBoundingClientRect();occupied.push({x:r.left-rootRect.left-8,y:r.top-rootRect.top-8,w:r.width+16,h:r.height+16});}
 imageCache.retain([...director.items.values()].map(r=>cover(r.p.w)).concat(hover?[cover(hover.w),avatar(hover.w)]:[]));
 director.update(buffer.filter(p=>timeMatches(p.w)),{now,width:W,height:HH,zoom:camera.zoom,blocked:occupied,placeRect:H.placeRect,cap:camera.zoom>1.65?13:9,selectedKey:null,hoverKey:hover?.w.key,advancing:orbit.phase==='rotating'||orbit.phase==='resuming',motion:worldRotation,reduced,ready:w=>!!image(cover(w)),failed:w=>imageCache.get(cover(w))?.failed});identityStats=director.snapshot();
 for(const [key,r]of director.items){const w=r.p.w;let card=identityCards.get(key);
  if(!card){card=make('button','sky-identity');card.type='button';card.dataset.key=key;card.setAttribute('aria-label',w.title||authorName(w)||'收藏封面');const art=make('span','identity-art'),img=make('img');img.alt='';img.referrerPolicy='no-referrer';const fallback=make('span','identity-fallback','▧');art.append(img,fallback);card.append(art);card.style.opacity='0';identityRoot.append(card);identityCards.set(key,card);}
  const url=cover(w),art=card.querySelector('img'),fallback=card.querySelector('.identity-fallback'),ready=image(url);if(ready){if(art.getAttribute('src')!==url)art.src=url;art.hidden=false;fallback.hidden=true;}else{art.hidden=true;fallback.hidden=false;fallback.title='封面未缓存';}
 }
}
function updateIdentityVisuals(){
 identityDrawn=identityCovers=identityMissing=0;
 for(const[key,card]of identityCards){const r=director.items.get(key);if(!r){card.remove();identityCards.delete(key);continue;}const p={...project(r.p.w.pos),w:r.p.w};r.p=p;const selected=key===selectedWork?.key;card.className='sky-identity'+(selected?' is-selected':'')+(!selected&&key!==hover?.w.key&&focusWeights.has(key)?' is-related':'');card.style.setProperty('--relation-alpha',String(.15+.7*Math.sqrt(focusWeights.get(key)||0)));card.style.width=r.w+'px';card.style.height=r.h+'px';card.style.transform=`translate(${p.x+r.dx}px,${p.y+r.dy}px)`;const depth=Math.max(.22,Math.min(1,(p.depth+1.5)/2.2));card.style.opacity=String(r.opacity*depth*layers.covers);card.style.pointerEvents=timeMatches(p.w)&&r.opacity*layers.covers>.55?'auto':'none';card.dataset.phase=r.phase;
  if(r.opacity>.02){identityDrawn++;if(!card.querySelector('img').hidden)identityCovers++;else identityMissing++;}
 }
}
on(identityRoot,'click',e=>{const card=e.target.closest('.sky-identity');if(!card)return;const w=M.byKey.get(card.dataset.key);if(w)openWork(w);});
on(identityRoot,'pointerover',e=>{if(flight.active)return;const card=e.target.closest('.sky-identity');if(!card||card.contains(e.relatedTarget))return;const w=M.byKey.get(card.dataset.key);if(!w||!timeMatches(w))return;const p=project(w.pos);hover={...p,w};mouse={x:p.x,y:p.y};interacted();showPreview(w);syncMotion();invalidate();});
on(identityRoot,'pointerout',e=>{const card=e.target.closest('.sky-identity');if(!card||card.contains(e.relatedTarget)||hover?.w.key!==card.dataset.key)return;interacted();hidePreview();});
function zoomAt(x,y,factor){cancelFlight();stopInertia();interacted();hidePreview();const world=H.unprojectPlane(target,x,y,W,HH);const next=boundCamera({...target,zoom:target.zoom*factor},W,HH);target=H.zoomAround(target,world,next.zoom,W,HH,{x,y});if(reduced)camera={...target};invalidate();}
on(canvas,'pointerdown',e=>{if(e.button!==0)return;cancelFlight();canvas.setPointerCapture(e.pointerId);interacted();stopInertia();target={...camera};drag={x:e.clientX,y:e.clientY,sx:e.clientX,sy:e.clientY,time:e.timeStamp,moved:false,shift:e.shiftKey,velocity:{yaw:0,pitch:0,panX:0,panY:0}};canvas.classList.add('dragging');hidePreview();});
on(canvas,'pointermove',e=>{if(!drag){hoverAt(e);return;}const dx=e.clientX-drag.x,dy=e.clientY-drag.y,dt=Math.max(.008,Math.min(.05,(e.timeStamp-drag.time)/1000));drag.x=e.clientX;drag.y=e.clientY;drag.time=e.timeStamp;if(Math.hypot(e.clientX-drag.sx,e.clientY-drag.sy)>4)drag.moved=true;
 if(drag.shift||e.shiftKey){camera.panX+=dx;camera.panY+=dy;drag.velocity.panX=drag.velocity.panX*.4+dx/dt*.6;drag.velocity.panY=drag.velocity.panY*.4+dy/dt*.6;}else{camera.yaw-=dx*.005;camera.pitch=Math.max(.2,Math.min(1.35,camera.pitch-dy*.004));drag.velocity.yaw=drag.velocity.yaw*.4+Math.max(-2.5,Math.min(2.5,-dx*.005/dt))*.6;drag.velocity.pitch=drag.velocity.pitch*.4+Math.max(-1.6,Math.min(1.6,-dy*.004/dt))*.6;}camera=boundCamera(camera,W,HH);target={...camera};invalidate();});
on(canvas,'pointerup',e=>{if(!drag)return;const d=drag;drag=null;interacted();syncMotion();canvas.classList.remove('dragging');canvas.releasePointerCapture(e.pointerId);if(d.moved){if(!reduced&&e.timeStamp-d.time<90)inertia={...d.velocity};invalidate();return;}const p=pickAt(e.clientX-root.getBoundingClientRect().left,e.clientY-root.getBoundingClientRect().top);if(p)openWork(p.w);});
on(canvas,'pointercancel',()=>{drag=null;stopInertia();interacted();hidePreview();canvas.classList.remove('dragging');});on(canvas,'pointerleave',()=>{if(!drag){mouse={x:-1000,y:-1000};if(hover){interacted();hidePreview();}}});
on(canvas,'lostpointercapture',()=>{if(drag){drag=null;stopInertia();canvas.classList.remove('dragging');interacted();hidePreview();}});on(window,'blur',()=>{cancelFlight();drag=null;stopInertia();mouse={x:-1000,y:-1000};canvas.classList.remove('dragging');interacted();hidePreview();});
on(root,'wheel',e=>{if(e.target.closest('[data-explore-details],.explore-controls,.explore-navigation,.explore-timeline'))return;e.preventDefault();zoomAt(e.clientX-root.getBoundingClientRect().left,e.clientY-root.getBoundingClientRect().top,H.wheelFactor(e.deltaY,e.deltaMode,e.ctrlKey));},{passive:false});
on(document,'keydown',e=>{if(root.closest('[inert]'))return;if(e.key==='Escape'){e.preventDefault();back();return;}if(e.target!==canvas)return;cancelFlight();let changed=false;if(e.key==='ArrowLeft'){target.yaw-=.12;changed=true;}if(e.key==='ArrowRight'){target.yaw+=.12;changed=true;}if(e.key==='ArrowUp'){target.pitch=Math.min(1.35,target.pitch+.08);changed=true;}if(e.key==='ArrowDown'){target.pitch=Math.max(.2,target.pitch-.08);changed=true;}if(e.key==='+'||e.key==='='){zoomAt(W*.53,HH*.515,1.2);changed=true;}if(e.key==='-'){zoomAt(W*.53,HH*.515,1/1.2);changed=true;}if(changed){e.preventDefault();interacted();invalidate();}});
on(document,'visibilitychange',()=>{syncMotion();interacted();if(document.hidden){cancelAnimationFrame(frame);frame=0;stopInertia();}else{last=performance.now();invalidate();}});on(reducedQuery,'change',e=>{reduced=e.matches;stopInertia();if(reduced){flight.cancel();camera={...target};}syncMotion();interacted();invalidate();});on(window,'resize',resize);const observer=new ResizeObserver(resize);observer.observe(root);resize();refresh();
options.onControls?.({setPublicationRange,isPaused:()=>userPaused,back,reset,zoomIn:()=>zoomAt(W*.5,HH*.515,1.25),zoomOut:()=>zoomAt(W*.5,HH*.515,.8),focusSelected:()=>{if(selectedWork){pushView();focusWork(selectedWork);}},toggleRotation:()=>{userPaused=!userPaused;interacted();syncMotion();invalidate();return userPaused;}});
const quantile=(a,p)=>{if(!a.length)return 0;const s=a.slice().sort((x,y)=>x-y);return +s[Math.min(s.length-1,Math.floor(s.length*p))].toFixed(2);};
window.__HOARD_QA__=()=>({viewport:{width:W,height:HH},points:import.meta.env.DEV?buffer.filter(p=>p.w.neighbors.length>=2&&p.x>50&&p.x<W-390&&p.y>55&&p.y<HH-55).slice(0,6).map(p=>({x:p.x,y:p.y,neighbors:p.w.neighbors.length})):undefined,identityPoints:import.meta.env.DEV?[...identityCards.keys()].map((key,index)=>{const p=buffer.find(p=>p.w.key===key);return p?{index,x:p.x,y:p.y,neighbors:p.w.neighbors.length}:null;}).filter(p=>p&&p.neighbors>=2):undefined,mountToken:H.hash(director.seed),layoutFingerprint,mode:'real',analysisVersion:D.analysisVersion,linkedWorks:M.works.filter(w=>w.neighbors.length).length,relations:relationStats,cameraFlight:flight.active,source,month:publicationSelection.range,matching:matchingKeys.size,active:active.length,rendered:stars.length,uniqueRendered:new Set(stars.map(w=>w.key)).size,workSelected:!!selectedWork,historyDepth:history.length,authorWorks:active.filter(w=>!!authorName(w)).length,coverWorks:active.filter(w=>!!cover(w)).length,metadataCoverage:D.meta.coverage,imageAssets:imageCounts(),loadedImages:[...imageCache.values()].filter(r=>r.ready).length,failedImages:[...imageCache.values()].filter(r=>r.failed).length,avatarDrawn:0,identities:{...director.snapshot(),covers:identityCovers,missing:identityMissing,cacheEntries:imageCache.size,cacheLimit:imageCache.limit,cacheEvictions:imageCache.evictions},identityTokens:import.meta.env.DEV?[...director.items].filter(([,r])=>r.opacity>.02).map(([key,r])=>({token:H.hash(key),x:+r.p.x.toFixed(2),y:+r.p.y.toFixed(2),opacity:+r.opacity.toFixed(3),phase:r.phase,selected:key===selectedWork?.key,hovered:key===hover?.w.key})):undefined,detailAvatar:{state:document.querySelector('[data-author-status]')?.dataset.state||'closed'},performance:{samples:frameSamples.length,drawMedianMs:quantile(frameSamples,.5),drawP95Ms:quantile(frameSamples,.95),rafMedianMs:quantile(frameGaps,.5),rafP95Ms:quantile(frameGaps,.95)},autoRotation:orbit.phase==='rotating'||orbit.phase==='resuming',motionState:orbit.phase,motionIdleMs:Math.round(performance.now()-orbit.lastActivity),motionFlags:{...orbit.flags},userPaused,worldRotation,cameraYaw:camera.yaw,cameraPitch:camera.pitch,cameraZoom:camera.zoom,targetZoom:target.zoom,minimumZoom:minimumImmersiveZoom(W,HH,camera.pitch),cameraPan:{x:camera.panX,y:camera.panY},inertia:{...inertia},layout:M.stats,neighborCount:selectedWork?.neighbors.length||0,reducedMotion:reduced});
return ()=>{rememberView(camera,worldRotation,userPaused);if(import.meta.env.DEV)console.debug("[interest-dev] page-view",JSON.stringify({phase:"save",camera,rotation:worldRotation,paused:userPaused}));options.onControls?.(null);disposed=true;observer.disconnect();cancelAnimationFrame(frame);clearTimeout(diagnosticTimer);for(const r of imageCache.values()){r.img.onload=null;r.img.onerror=null;}imageCache.clear();imageStates.clear();listeners.forEach(remove=>remove());root.replaceChildren();delete window.__HOARD_QA__;};
}
