// Bounded decoded images, not a lifetime request quota. Active/fading covers stay pinned.
export class ImagePool {
 constructor(limit=128,onEvict=()=>{}){this.limit=limit;this.map=new Map();this.pins=new Set();this.evictions=0;this.onEvict=onEvict;}
 get size(){return this.map.size;}
 has(key){return this.map.has(key);}
 get(key){const v=this.map.get(key);if(v){this.map.delete(key);this.map.set(key,v);}return v;}
 values(){return this.map.values();}
 retain(keys){this.pins=new Set(keys);}
 set(key,value){if(this.map.has(key)){this.map.delete(key);this.map.set(key,value);return true;}
  if(this.map.size>=this.limit){const victim=[...this.map].find(([k,v])=>!this.pins.has(k)&&(v.ready||v.failed));if(!victim)return false;this.map.delete(victim[0]);this.onEvict(victim[1]);this.evictions++;}
  this.map.set(key,value);return true;
 }
 clear(){for(const value of this.map.values())this.onEvict(value);this.map.clear();this.pins.clear();}
}
