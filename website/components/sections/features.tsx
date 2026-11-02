import { LibraryPreview } from "@/components/library-preview";
import type { Language } from "@/lib/content";

export function Features({ language }: { language: Language }) {
  return <section className="py-8"><LibraryPreview language={language} /></section>;
}
