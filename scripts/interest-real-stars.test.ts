import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import geometry from "../web/src/pages/constellation-dev/core.js";

test("star population contains real source-scoped works only, including isolated/no-text works", () => {
 const scopes = ["douyin", "bilibili"].map(source => ({source,
  topics: [{id:"t_uncategorized",label:"未关联",kind:"bucket"},{id:"t_no_text",label:"无文案",kind:"bucket"}],
  topicMembership: [{workId:"same-id",topicId:"t_uncategorized",evidence:[]},{workId:"empty",topicId:"t_no_text",evidence:[]}],
  works: [{id:"same-id",anchorTopicId:"t_uncategorized",title:"孤立作品"},{id:"empty",anchorTopicId:"t_no_text",title:""}],
 }));
 const model=geometry.prepare({schemaVersion:2,scopes});
 expect(model.works).toHaveLength(4);
 expect(model.byKey.size).toBe(4);
 const sampled=geometry.sample(model.works,3000);
 expect(sampled).toHaveLength(4);
 for(const w of sampled){
  expect(model.byKey.get(w.key)).toBe(w);
  expect(Number.isFinite(w.pos.x)&&Number.isFinite(w.pos.y)&&Number.isFinite(w.pos.z)).toBe(true);
 }
 expect(geometry.prepare({schemaVersion:2,scopes:[]} ).works).toHaveLength(0);
});

test("renderer has no decorative stars, dust, or artificial topic fog", () => {
 const renderer=readFileSync("web/src/pages/constellation-dev/renderer.js","utf8");
 expect(renderer).not.toMatch(/\b(bg|dust|mist)\b|disk-a|bgx|fog/);
 expect(renderer).toContain("buffer=stars.map(w=>");
 expect(renderer).toContain("for(const p of buffer){const w=p.w");
 expect(renderer).toContain("H.pick(buffer,x,y,11)");
 // Only background surfaces may use rectangles; all point primitives use a real work.
 expect((renderer.match(/ctx.fillRect\(/g)||[]).length).toBe(3);
 expect(renderer).not.toContain("H.mst(");
});

test("many text-derived groups remain inside the sky rather than unbounded preset-topic rings", () => {
 const topics=Array.from({length:200},(_,i)=>({id:`local-${i}`,label:`shared-${i}`,kind:"topic"}));
 const works=topics.map((t,i)=>({id:`w-${i}`,anchorTopicId:t.id,title:`synthetic-${i}`}));
 const topicMembership=works.map(w=>({workId:w.id,topicId:w.anchorTopicId,evidence:[]}));
 const model=geometry.prepare({schemaVersion:2,scopes:[{source:"douyin",topics,works,topicMembership}]});
 expect(model.works).toHaveLength(200);
 for(const t of model.topics.values())expect(Math.hypot(t.center.x,t.center.y)).toBeLessThanOrEqual(1);
 for(const w of model.works)expect(Math.hypot(w.pos.x,w.pos.y)).toBeLessThan(1.4);
});

test("sparse pair weights affect proximity without inventing points or crossing sources", () => {
 const scope=(source:string,neighbors:unknown[])=>({source,neighbors,
  topics:[{id:`${source}-local`,label:"synthetic",kind:"topic"}],
  works:[{id:"a",anchorTopicId:`${source}-local`},{id:"b",anchorTopicId:`${source}-local`}],
  topicMembership:[{workId:"a",topicId:`${source}-local`,evidence:[]},{workId:"b",topicId:`${source}-local`,evidence:[]}],
 });
 const scopes=[scope("douyin",[]),scope("bilibili",[])];
 const base=geometry.prepare({schemaVersion:2,scopes});
 const linked=geometry.prepare({schemaVersion:2,scopes:[scope("douyin",[{workId:"a",to:"b",score:.8},{workId:"b",to:"a",score:.8}]),scopes[1]]});
 const distance=(m:typeof base)=>{const a=m.byKey.get("douyin:a").pos,b=m.byKey.get("douyin:b").pos;return Math.hypot(a.x-b.x,a.y-b.y,a.z-b.z);};
 expect(distance(linked)).toBeLessThan(distance(base));
 expect(linked.works).toHaveLength(4);
 expect(linked.byKey.get("bilibili:a").pos).toEqual(base.byKey.get("bilibili:a").pos);
});
