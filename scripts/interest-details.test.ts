import { expect, test, mock } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
// Unit-test the panel shell without booting the native-only library/i18n bridge.
mock.module("../web/src/pages/constellation-dev/work-list",()=>({SnapshotWorkList:({view,hidden}:{view:PanelView;hidden:boolean})=>hidden?null:createElement("div",null,view.works.map(w=>createElement("article",{key:w.key},w.title,w.author)))}));
mock.module("../web/src/components/viewer/info-panel",()=>({InfoPanel:({entry}:{entry:{title:string;author:string}})=>createElement("div",null,entry.title,entry.author)}));
const {ContentDetails}=await import("../web/src/pages/constellation-dev/details");
import type { PanelView } from "../web/src/pages/constellation-dev/view-types";
const item={key:"synthetic:1",id:"1",sourceCode:"douyin",kind:"video" as const,title:"测试收藏标题",source:"抖音",author:"测试作者",avatar:"hoard-cover://localhost/a/fixture?cacheOnly=1",cover:"hoard-cover://localhost/w/fixture?cacheOnly=1",topics:[{id:"topic-photo",label:"摄影"}]};
const view:PanelView={key:"fixture-snapshot:topic",label:"摄影",work:null,works:[item],back:()=>{},openWork:()=>{},focusTheme:()=>{},imageStatus:()=>{}};
for (const mode of ["group","work"] as const) {
 test(`Hoard ${mode} details contain only content and existing controls`,()=>{
  const html=renderToStaticMarkup(createElement(ContentDetails,{view:{...view,work:mode==="work"?item:null}}));
  expect(html).toContain("测试收藏标题");expect(html).toContain("测试作者");if(mode==="group")expect(html).toContain("摄影");else{expect(html).not.toContain("摄影");expect(html).toContain("相关作品");}expect(html).toContain('data-slot="button"');expect(html).toContain("lucide-x");
  expect(html).not.toMatch(/<details|<summary|关于|主归属|画布显示|规则|置信|抽样|未归类|开发|tooltip/);
 });
}
test("no selection means no side panel",()=>{expect(renderToStaticMarkup(createElement(ContentDetails,{view:null}))).toBe("")});
test('isolated selection has no fabricated related library or topic chips',()=>{
 const html=renderToStaticMarkup(createElement(ContentDetails,{view:{...view,work:item,works:[]}}));
 expect(html).toContain('暂无关联作品');expect(html).not.toContain('<article');expect(html).not.toContain('摄影');
});
