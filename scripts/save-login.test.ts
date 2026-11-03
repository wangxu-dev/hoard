import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";

const script = readFileSync(new URL("../internal/douyin/save-login.js", import.meta.url), "utf8");
async function run(labels: string[], navigate = false, checked?: string, persistent?: boolean) {
  let now = 0;
  let shown = true;
  let clicks = 0;
  const buttons = labels.map((label) => ({
    isConnected: true, innerText: label, disabled: false,
    getClientRects: () => shown ? [1] : [], getAttribute: () => null,
    click: () => { clicks++; shown = false; },
  }));
  const control = { isConnected: true, getClientRects: () => [1], getAttribute: (name: string) => name === 'aria-checked' ? checked : null };
  const prompt = { isConnected: true, innerText: "保存登录信息", getClientRects: () => shown ? [1] : [], querySelectorAll: (selector: string) => selector.includes('switch') ? (checked === undefined ? [] : [control]) : buttons, contains: () => true };
  const location = { href: "https://www.douyin.com/user/self" };
  const result = await runInNewContext(script, {
    Date: { now: () => now }, location,
    document: { querySelectorAll: (selector: string) => selector === '.trust-login-switch-button' ? (persistent === undefined ? [] : [{ ...control, classList: { contains: () => persistent } }]) : labels.length || checked !== undefined ? [prompt] : [] },
    getComputedStyle: () => ({ visibility: "visible" }),
    setTimeout: (callback: () => void) => { now += 250; if (navigate) location.href += "#changed"; queueMicrotask(callback); },
  });
  return { result, clicks };
}

test("save-login accepts save and confirm, and absence of a prompt", async () => {
  for (const label of ["保存", "确定", "保存登录信息"]) expect(await run([label])).toEqual({ result: true, clicks: 1 });
  expect(await run([])).toEqual({ result: true, clicks: 0 });
});
test("save-login recognizes the persistent switch without clicking it", async () => {
  expect(await run([], false, undefined, true)).toEqual({ result: true, clicks: 0 });
  expect(await run([], false, undefined, false)).toEqual({ result: false, clicks: 0 });
  expect(await run([], false, 'true')).toEqual({ result: true, clicks: 0 });
  expect(await run([], false, 'false')).toEqual({ result: false, clicks: 0 });
  expect(await run([], false, 'mixed')).toEqual({ result: false, clicks: 0 });
});

test("save-login leaves ambiguous or unrecognized prompts for the user", async () => {
  expect(await run(["保存", "确定"])).toEqual({ result: false, clicks: 0 });
  expect(await run(["取消"])).toEqual({ result: false, clicks: 0 });
  expect((await run([], true)).result).toBe(false);
});
