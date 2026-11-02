package main

// Only known UI labels and sanitized API route names leave the page.
type spaceSummary struct {
	Stage    string         `json:"stage"`
	Labels   []string       `json:"labels"`
	Requests []spaceRequest `json:"requests"`
}
type spaceRequest struct {
	Path       string   `json:"path"`
	Parameters []string `json:"parameters"`
}

const exploreScript = `(async()=>{
 if(location.hostname==='www.bilibili.com'){
  const response=await fetch('https://api.bilibili.com/x/web-interface/nav',{credentials:'include',signal:AbortSignal.timeout(10000)});
  const json=await response.json();
  if(json.code===0&&json.data?.isLogin){location.href='https://space.bilibili.com/'+String(json.data.mid);return {stage:'opening-space'}};
  return {stage:'waiting-login'};
 }
 if(location.hostname!=='space.bilibili.com')return {stage:'other-page'};
 const labels=['主页','动态','投稿','合集和列表','收藏','收藏夹','最近点赞','我的点赞','喜欢','点赞','下一页','个人中心'];
 const nodes=[...document.querySelectorAll('a,button,[role=tab],span,h2,h3,div')].filter(el=>el.children.length<3&&labels.some(label=>el.textContent.trim().startsWith(label)&&el.textContent.trim().length<24)&&el.getBoundingClientRect().width>0);
 const labelOf=el=>labels.find(label=>el.textContent.trim().startsWith(label));
 const requests=performance.getEntriesByType('resource').flatMap(entry=>{
  try{const url=new URL(entry.name);if(url.hostname!=='api.bilibili.com'||!/^\/x\/[a-z0-9/_-]+$/i.test(url.pathname))return [];
   return [{path:url.pathname,parameters:[...url.searchParams.keys()].filter(k=>/^[a-z_]{1,40}$/.test(k))}];
  }catch{return []}
 });
 if(!window.__hoardSpaceClicked){
 window.scrollBy(0,600);
  const likes=nodes.find(el=>['最近点赞','我的点赞','点赞'].includes(labelOf(el)));
  if(likes){window.__hoardSpaceClicked=true;likes.click();return {stage:'clicked-likes',labels:[...new Set(nodes.map(labelOf))],requests}}
 }
 return {stage:'space',labels:[...new Set(nodes.map(labelOf))],requests};
})()`
