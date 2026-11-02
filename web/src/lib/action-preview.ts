import { useSyncExternalStore } from "react";
import type { ListKind } from "./library";

export type ActionKind = ListKind;
export interface ActionTarget { id: string; kind: ActionKind }
const overrides = new Map<string, readonly string[]>();
const listeners = new Set<() => void>();
const defaults = new Map<string, readonly string[]>();
const keyOf = (target: ActionTarget) => `${target.kind}:${target.id.replace(/^demo-(like|favorite)-/, "demo-work-")}`;

// Experimental state is in memory only; it never invokes a platform API.
export function previewOwners(target: ActionTarget): readonly string[] {
  const key = keyOf(target);
  const saved = overrides.get(key) ?? defaults.get(key);
  if (saved) return saved;
  const n = Number(target.id.match(/\d+$/)?.[0] ?? 0);
  const owners = target.kind === "favorite" ? ["demo-b"] : n % 3 === 0 ? ["demo-a", "demo-b"] : ["demo-a"];
  defaults.set(key, owners);
  return owners;
}
export function setPreviewRelation(target: ActionTarget, accountId: string, on: boolean) {
  if (!["demo-a", "demo-b"].includes(accountId)) throw new Error("INVALID_ACCOUNT");
  const owners = previewOwners(target).filter((id) => id !== accountId);
  overrides.set(keyOf(target), on ? [...owners, accountId].sort() : owners);
  listeners.forEach((listener) => listener());
}
export function usePreviewOwners(target: ActionTarget) {
  return useSyncExternalStore((listener) => { listeners.add(listener); return () => { listeners.delete(listener); }; }, () => previewOwners(target));
}
