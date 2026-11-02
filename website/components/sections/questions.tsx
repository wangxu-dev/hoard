import { Faq } from "@/components/faq";
import { dictionaries, type Language } from "@/lib/content";

export function Questions({ language }: { language: Language }) {
  const text = dictionaries[language];
  return (
    <section
      id="faq"
      className="site-container faq-section section-space border-t"
    >
      <div>
        <p className="eyebrow">{text.faq.eyebrow}</p>
        <h2 className="section-title">{text.faq.title}</h2>
      </div>
      <Faq language={language} />
    </section>
  );
}
