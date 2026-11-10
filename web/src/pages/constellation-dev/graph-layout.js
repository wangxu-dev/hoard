// Independent sparse graph layout. Community ids are geometry only, never content topics.
// Weighted modularity local moves: O(passes * (V+E)). Spatial hashes bound collision work.
export function graphLayout(works, pairs, H) {
 const started=performance.now(), n=works.length, index=new Map(works.map((w,i)=>[w.key,i]));
 const adj=Array.from({length:n},()=>new Map()),degree=new Float64Array(n);
 for(const {a,b,weight} of pairs.values()){
  const i=index.get(a.key),j=index.get(b.key);adj[i].set(j,weight);adj[j].set(i,weight);degree[i]+=weight;degree[j]+=weight;
 }
 const total=degree.reduce((a,b)=>a+b,0),label=Int32Array.from({length:n},(_,i)=>i),mass=Float64Array.from(degree);
 const order=works.map((_,i)=>i).filter(i=>degree[i]>0).sort((a,b)=>degree[b]-degree[a]||works[a].key.localeCompare(works[b].key));
 // Only a real edge can cause a move. Modularity's null-model term prevents a giant component
 // becoming one unstructured ball; no title, keyword, author or service category is consulted.
 for(let pass=0;pass<16&&total;pass++){
  let moved=0;
  for(const i of order){
   const before=label[i],d=degree[i],weights=new Map();mass[before]-=d;
   for(const [j,w]of adj[i])weights.set(label[j],(weights.get(label[j])||0)+w);
   let best=before,bestGain=(weights.get(before)||0)-d*mass[before]/total;
   for(const [c,w]of weights){const gain=w-d*mass[c]/total;if(gain>bestGain+1e-9||(Math.abs(gain-bestGain)<1e-9&&c<best)){best=c;bestGain=gain;}}
   label[i]=best;mass[best]+=d;if(best!==before)moved++;
  }
  if(!moved)break;
 }
 const groups=new Map();
 for(const i of order){const c=label[i];if(!groups.has(c))groups.set(c,{key:works[i].key,indices:[],radius:0,center:null});const g=groups.get(c);g.indices.push(i);if(works[i].key<g.key)g.key=works[i].key;}
 const communities=[...groups.values()].sort((a,b)=>b.indices.length-a.indices.length||a.key.localeCompare(b.key));
 // Coherent connected components hold their graph-derived subcommunities together.
 // Without this hierarchy the large component was dispersed into dozens of unrelated islands.
 const parent=Int32Array.from({length:n},(_,i)=>i),root=i=>{while(parent[i]!==i){parent[i]=parent[parent[i]];i=parent[i];}return i;};
 for(const i of order)for(const j of adj[i].keys()){let a=root(i),b=root(j);if(a!==b)parent[Math.max(a,b)]=Math.min(a,b);}
 const components=new Map();
 for(const g of communities){const k=root(g.indices[0]);if(!components.has(k))components.set(k,{key:works[k].key,groups:[],size:0});const c=components.get(k);c.groups.push(g);c.size+=g.indices.length;g.radius=.009+.008*Math.sqrt(g.indices.length);}
 const ranked=[...components.values()].sort((a,b)=>b.size-a.size||a.key.localeCompare(b.key));
 const cell=.08,grid=new Map();
 // Index circle bounds, not just centers, so queries need no global max-radius scan.
 const put=g=>{for(let x=Math.floor((g.center.x-g.radius)/cell);x<=Math.floor((g.center.x+g.radius)/cell);x++)for(let y=Math.floor((g.center.y-g.radius)/cell);y<=Math.floor((g.center.y+g.radius)/cell);y++){const k=x+','+y;if(!grid.has(k))grid.set(k,[]);grid.get(k).push(g);}};
 const overlap=(x,y,r)=>{const seen=new Set();let penalty=0;for(let a=Math.floor((x-r-.008)/cell);a<=Math.floor((x+r+.008)/cell);a++)for(let b=Math.floor((y-r-.008)/cell);b<=Math.floor((y+r+.008)/cell);b++)for(const g of grid.get(a+','+b)||[]){if(seen.has(g))continue;seen.add(g);penalty+=Math.max(0,r+g.radius+.008-Math.hypot(x-g.center.x,y-g.center.y));}return penalty;};
 ranked.forEach((c,k)=>{
  c.radius=.016+.011*Math.sqrt(c.size);let best=null;
  for(let attempt=0;attempt<64;attempt++){
   const u=H.rnd(c.key+':pack:'+attempt),r=k===0&&c.size>100?.12:(.10+1.0*Math.pow(u,.58)),arm=k%3;
   const a=arm*Math.PI*2/3+3.5*r+H.normal(c.key+':turn:'+attempt)*(.13+attempt*.005);
   const x=Math.cos(a)*r,y=Math.sin(a)*r,penalty=overlap(x,y,c.radius);
   if(!best||penalty<best.penalty)best={x,y,z:H.normal(c.key+':height')*.018,penalty};if(penalty===0)break;
  }
  c.center={x:best.x,y:best.y,z:best.z};put(c);
  c.groups.sort((a,b)=>b.indices.length-a.indices.length||a.key.localeCompare(b.key));
  const offset=H.rnd(c.key+':inside')*Math.PI*2;
  c.groups.forEach((g,j)=>{const a=offset+j*2.3999632297,r=c.groups.length===1?0:(c.radius-g.radius)*Math.sqrt((j+.25)/c.groups.length);g.center={x:c.center.x+Math.cos(a)*r,y:c.center.y+Math.sin(a)*r,z:c.center.z+H.normal(g.key+':height')*.015};});
 });
 // Spatially local repulsion and cross-community edge springs inside each component.
 const byLabel=new Map(communities.map(g=>[label[g.indices[0]],g]));
 const bridges=new Map();
 for(const {a,b,weight} of pairs.values()){
  const ga=byLabel.get(label[index.get(a.key)]),gb=byLabel.get(label[index.get(b.key)]);if(ga===gb)continue;
  const key=ga.key<gb.key?ga.key+'|'+gb.key:gb.key+'|'+ga.key,edge=bridges.get(key);if(edge)edge.weight+=weight;else bridges.set(key,{a:ga,b:gb,weight});
 }
 for(let step=0;step<24;step++){
  const forces=new Map(communities.map(g=>[g,{x:0,y:0}]));
  for(const c of ranked){const bins=new Map(),unit=.08;
   for(const g of c.groups){const k=Math.floor(g.center.x/unit)+','+Math.floor(g.center.y/unit);if(!bins.has(k))bins.set(k,[]);bins.get(k).push(g);}
   for(const g of c.groups){const cx=Math.floor(g.center.x/unit),cy=Math.floor(g.center.y/unit),range=Math.ceil((g.radius+.12)/unit),f=forces.get(g);
    for(let x=cx-range;x<=cx+range;x++)for(let y=cy-range;y<=cy+range;y++)for(const other of bins.get(x+','+y)||[]){if(other===g)continue;const dx=g.center.x-other.center.x,dy=g.center.y-other.center.y,d=Math.max(.0001,Math.hypot(dx,dy)),wanted=(g.radius+other.radius)*.9;if(d<wanted){f.x+=dx/d*(wanted-d)*.14;f.y+=dy/d*(wanted-d)*.14;}}
   }
  }
  for(const e of bridges.values()){const dx=e.b.center.x-e.a.center.x,dy=e.b.center.y-e.a.center.y,d=Math.max(.0001,Math.hypot(dx,dy)),rest=e.a.radius+e.b.radius+.012,pull=(d-rest)*.012*Math.min(3,Math.sqrt(e.weight))/d;forces.get(e.a).x+=dx*pull;forces.get(e.a).y+=dy*pull;forces.get(e.b).x-=dx*pull;forces.get(e.b).y-=dy*pull;}
  for(const c of ranked)for(const g of c.groups){const f=forces.get(g);g.center.x+=Math.max(-.008,Math.min(.008,f.x));g.center.y+=Math.max(-.008,Math.min(.008,f.y));const dx=g.center.x-c.center.x,dy=g.center.y-c.center.y,d=Math.hypot(dx,dy),limit=c.radius-g.radius*.5;if(d>limit){g.center.x=c.center.x+dx*limit/d;g.center.y=c.center.y+dy*limit/d;}}
 }
 communities.forEach(g=>{
  g.indices.sort((a,b)=>degree[b]-degree[a]||works[a].key.localeCompare(works[b].key));
  const offset=H.rnd(g.key+':cloud')*Math.PI*2;
  g.indices.forEach((i,j)=>{
   const r=g.radius*.83*Math.sqrt((j+.5)/g.indices.length),a=offset+j*2.3999632297;
   works[i].pos={x:Math.cos(a)*r,y:Math.sin(a)*r,z:H.normal(works[i].key+':localZ')*.009};
   works[i].community=g.key;
  });
  // Edge springs plus short-range repulsion, not repeated averaging. Preserve readable spread.
  const members=new Set(g.indices),spacing=.011;
  for(let step=0;step<28;step++){
   const near=new Map(),force=new Map();
   for(const i of g.indices){const p=works[i].pos,k=Math.floor(p.x/spacing)+','+Math.floor(p.y/spacing);if(!near.has(k))near.set(k,[]);near.get(k).push(i);force.set(i,{x:-p.x*.025,y:-p.y*.025});}
   for(const i of g.indices){const p=works[i].pos,f=force.get(i),cx=Math.floor(p.x/spacing),cy=Math.floor(p.y/spacing);
    for(let x=cx-1;x<=cx+1;x++)for(let y=cy-1;y<=cy+1;y++)for(const j of near.get(x+','+y)||[]){if(i===j)continue;const q=works[j].pos,dx=p.x-q.x,dy=p.y-q.y,d=Math.max(.0001,Math.hypot(dx,dy));if(d<spacing){const push=(spacing-d)*.28/d;f.x+=dx*push;f.y+=dy*push;}}
    for(const [j,w]of adj[i])if(members.has(j)){const q=works[j].pos,dx=q.x-p.x,dy=q.y-p.y,d=Math.max(.0001,Math.hypot(dx,dy)),rest=spacing*(1.1+(1-w)*1.8),pull=(d-rest)*.07*w/d/Math.sqrt(adj[i].size);f.x+=dx*pull;f.y+=dy*pull;}
   }
   for(const i of g.indices){const p=works[i].pos,f=force.get(i);p.x+=Math.max(-.008,Math.min(.008,f.x));p.y+=Math.max(-.008,Math.min(.008,f.y));const r=Math.hypot(p.x,p.y),limit=g.radius;if(r>limit){p.x*=limit/r;p.y*=limit/r;}}
  }
  for(const i of g.indices){const w=works[i];w.pos.x+=g.center.x;w.pos.y+=g.center.y;w.pos.z+=g.center.z;w.color=H.palette[H.hash(g.key)%H.palette.length];}
 });
 for(let i=0;i<n;i++)if(!degree[i]){
  const w=works[i],r=1.24*Math.pow(H.rnd(w.key+':fieldR'),.65),a=(H.hash(w.key)%3)*Math.PI*2/3+3.5*r+H.normal(w.key+':fieldA')*.32;
  w.pos={x:Math.cos(a)*r,y:Math.sin(a)*r,z:H.normal(w.key+':fieldZ')*(.025+r*.025)};w.community=null;w.color='#8196aa';
 }
 return {communities:communities.map(g=>({key:g.key,size:g.indices.length,center:g.center,radius:g.radius})),stats:{works:n,linked:order.length,isolated:n-order.length,pairs:pairs.size,communities:communities.length,components:ranked.length,elapsedMs:Math.round(performance.now()-started)}};
}
