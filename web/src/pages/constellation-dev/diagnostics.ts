// Closed-vocabulary messages: never log raw bridge errors, identifiers, URLs or paths.
const codes=["STORE_NOT_READY","INVALID_REQUEST","INVALID_SCOPE","SCOPE_MISMATCH","TASK_START_FAILED","TASK_NOT_FOUND","TASK_NOT_COMPLETE","TASK_SUPERSEDED","SERVICE_CLOSED","SNAPSHOT_MISSING","SNAPSHOT_INVALID","SNAPSHOT_TIME_INVALID","SNAPSHOT_COUNTS_INVALID","SNAPSHOT_WORK_INVALID","SNAPSHOT_ASSET_INVALID","SNAPSHOT_MEMBERSHIP_INVALID","SNAPSHOT_TOPIC_INVALID","SNAPSHOT_ANCHOR_INVALID","SNAPSHOT_SCOPE_INVALID","SNAPSHOT_TOO_LARGE","SNAPSHOT_INCONSISTENT","EXPLORATION_FAILED","CANCELLED"];
const phrases=["access is denied","permission denied", "no space left", "disk full", "sharing violation", "磁盘空间不足", "另一个程序正在使用", "not a function", "unknown service", "operation not permitted", "拒绝访问", "系统找不到指定", "不支持", "未注册", "数据库已锁定","no such file","not found","not registered","not initialized","not ready","invalid handle","incorrect function","not supported", "unsupported value","read-only","readonly","locked","busy","timed out","timeout","cancelled","canceled","closed","transport","failed to fetch","failed to execute","method","binding","sqlite","argument","undefined","null"];
export function diagnosticError(error?:unknown) {
 const value=error as {message?:unknown;name?:unknown}|undefined;
 const message=typeof error==="string"?error:typeof value?.message==="string"?value.message:"";
 const code=codes.find(c=>new RegExp(`(?:^|[^A-Z_])${c}(?:$|[^A-Z_])`).test(message))||(error?"BRIDGE_OR_TRANSPORT":"UNKNOWN");
 const name=value?.name;const kind=["Error","TypeError","AbortError"].includes(String(name))?String(name):typeof error==="string"?"string":error?"object":"none";
 return {code,kind,message:phrases.filter(p=>message.toLowerCase().includes(p)).join("; ")||"redacted"};
}
