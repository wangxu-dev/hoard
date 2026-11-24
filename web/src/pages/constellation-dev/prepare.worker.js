import H from "./core.js";
import { RelationLayer } from "./relation-lines.js";

self.onmessage = (event) => {
  try {
    const prepared = H.prepare(event.data);
    prepared.initialRelations = new RelationLayer(prepared.works, prepared.byKey).background;
    self.postMessage(prepared);
  } catch {
    self.postMessage({ error: true });
  }
};
