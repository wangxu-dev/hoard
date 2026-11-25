import H from "./core";
import { RelationLayer } from "./relation-lines";
import type { ExplorationInput } from "./types";

self.onmessage = (event: MessageEvent<ExplorationInput>) => {
  try {
    const prepared = H.prepare(event.data);
    prepared.initialRelations = new RelationLayer(prepared.works, prepared.byKey).background;
    self.postMessage(prepared);
  } catch {
    self.postMessage({ error: true });
  }
};
