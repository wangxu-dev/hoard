import type { PanelView } from "./view-types";
import type { TimeSelection } from "./publication-time";
export interface ViewControls { setPublicationRange: (selection: TimeSelection) => void; isPaused: () => boolean; back: () => void; reset: () => void; zoomIn: () => void; zoomOut: () => void; focusSelected: () => void; toggleRotation: () => boolean; }
export function mountInterestSky(root: HTMLElement, data: unknown, options?: { onPanel?: (view: PanelView | null) => void; onControls?: (controls: ViewControls | null) => void }): () => void;
