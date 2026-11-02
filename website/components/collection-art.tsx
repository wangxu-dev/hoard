import { DemoFrame } from "@/components/demo-frame";
import type { Language } from "@/lib/content";

export function CollectionArt({ language }: { language: Language }) {
  return (
    <DemoFrame
      language={language}
      collection
      className="collection-art block border-0"
    />
  );
}
