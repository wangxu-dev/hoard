// Independently painted, continuous background light. These are NOT work/star nodes.
// Every patch is projected on the same rotating galactic plane as the real works.
export function atmospherePatches(){
 const patches=[{x:0,y:0,z:-.025,r:.34,color:0,alpha:.36},{x:.025,y:-.015,z:0,r:.16,color:1,alpha:.62}];
 for(let arm=0;arm<3;arm++)for(let j=0;j<52;j++){
  const r=.12+j*.030,a=arm*Math.PI*2/3+3.6*r+.18*Math.sin(j*.24+arm),width=.065+.040*r;
  patches.push({x:Math.cos(a)*r,y:Math.sin(a)*r,z:-.02+.009*Math.sin(j*.32),r:width,color:r<.3?1:r<.78?0:2,alpha:.24*(1-r*.35)*(arm===0?1:arm===1?.8:.65)});
 }
 return patches;
}
export function createAtmosphere(){
 const colors=['103,176,184','225,207,167','122,137,185'];
 const textures=colors.map(color=>{const c=document.createElement('canvas');c.width=c.height=192;const ctx=c.getContext('2d'),g=ctx.createRadialGradient(96,96,0,96,96,96);g.addColorStop(0,`rgba(${color},.85)`);g.addColorStop(.36,`rgba(${color},.45)`);g.addColorStop(1,`rgba(${color},0)`);ctx.fillStyle=g;ctx.fillRect(0,0,192,192);return c;});
 const patches=atmospherePatches();
 return {draw(ctx,project,width,height){
  ctx.save();ctx.globalCompositeOperation='screen';
  for(const p of patches){const c=project(p),x=project({x:p.x+p.r,y:p.y,z:p.z}),y=project({x:p.x,y:p.y+p.r,z:p.z});const extent=Math.max(Math.hypot(x.x-c.x,x.y-c.y),Math.hypot(y.x-c.x,y.y-c.y))*1.8;if(c.x+extent<0||c.x-extent>width||c.y+extent<0||c.y-extent>height)continue;
   ctx.save();ctx.globalAlpha=p.alpha;ctx.transform(x.x-c.x,x.y-c.y,y.x-c.x,y.y-c.y,c.x,c.y);ctx.drawImage(textures[p.color],-1,-1,2,2);ctx.restore();
  }
  ctx.restore();
 }};
}
