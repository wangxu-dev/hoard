import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";
import {diagnosticError} from "../web/src/pages/constellation-dev/diagnostics";
test("bridge errors retain useful safe detail, never identifiers, URLs, paths or tokens",()=>{
 for(const value of [new Error("sync C:\\Users\\private-name\\private-title: Access is denied token=private-token"),{name:"private-name",message:"method not found private-account https://private.invalid/session"},"SQLITE_BUSY private-title; 拒绝访问"]){
  const out=JSON.stringify(diagnosticError(value));expect(out).not.toMatch(/private-|C:|Users|https:|session|token=/);expect(out).not.toContain('"message":"redacted"');
 }
 expect(diagnosticError(new Error("TASK_NOT_FOUND"))).toMatchObject({code:"TASK_NOT_FOUND",kind:"Error"});expect(diagnosticError(new Error("姓名 secret-token"))).toMatchObject({message:"redacted"});
});
test("request and each rejecting bridge phase are logged through existing App service",()=>{
 const load=readFileSync("web/src/pages/constellation-dev/load.ts","utf8");for(const s of ["request","start-call","start-reject","status-reject","result-reject","scope-reject","late-drop","abort"])expect(load).toContain(`"${s}"`);
 expect(load).toContain("App.reportExplorationDiagnostic(data)");expect(load).toContain("native-log-unavailable");expect(load).toContain("Promise.resolve().then(()=>App.reportExplorationDiagnostic(data))");expect(load).not.toMatch(/console\.(?:warn|error)\([^\n]*error\.message/);
});
