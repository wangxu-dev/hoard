// Only a user-opened detail may use the normal native avatar resolver. The galaxy
// still reads cache-only covers; there is no full-library image prefetch/fill.
export function detailAvatarURL(ref:string):string{
 if(!ref)return '';
 try{const u=new URL(ref);const native=(u.protocol==='hoard-cover:'&&u.hostname==='localhost')||((u.protocol==='http:'||u.protocol==='https:')&&u.hostname==='hoard-cover.localhost');
  if(!native||!/^\/a\/[^/]+$/.test(u.pathname)||u.username||u.password||u.port||u.hash)return '';
  u.searchParams.delete('cacheOnly');return u.href;
 }catch{return '';}
}
