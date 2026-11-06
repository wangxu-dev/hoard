import type { PanelView } from "./view-types";
export function mountInterestSky(root: HTMLElement, data: unknown, options?: { onPanel?: (view: PanelView | null) => void }): () => void;
