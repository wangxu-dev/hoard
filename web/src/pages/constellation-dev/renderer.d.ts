import type { PanelView } from "./view-types";
export interface ViewControls { back: () => void; reset: () => void; zoomIn: () => void; zoomOut: () => void; focusSelected: () => void; toggleRotation: () => boolean; }
export function mountInterestSky(root: HTMLElement, data: unknown, options?: { onPanel?: (view: PanelView | null) => void; onControls?: (controls: ViewControls | null) => void }): () => void;
