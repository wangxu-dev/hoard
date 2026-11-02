import { Header, Footer } from "@/components/shell";
import { dictionaries, homePath, type Language } from "@/lib/content";
import { repository } from "@/lib/site";
import { ArrowLeft, ArrowUpRight } from "lucide-react";

export function Privacy({ language }: { language: Language }) {
  const text = dictionaries[language];
  return (
    <>
      <Header language={language} privacy />
      <main id="main" className="site-container privacy-page section-space">
        <a
          href={homePath(language)}
          className="mb-10 inline-flex items-center gap-2 text-sm text-muted-foreground"
        >
          <ArrowLeft className="size-4" />
          {text.nav.home}
        </a>
        <h1 className="section-title">{text.privacy.title}</h1>
        <p className="mt-6 text-lg text-muted-foreground">
          {text.privacy.intro}
        </p>
        <div className="mt-12 space-y-10">
          {text.privacy.sections.map((section) => (
            <section key={section.title}>
              <h2 className="text-xl font-semibold">{section.title}</h2>
              <p className="mt-3 leading-8 text-muted-foreground">
                {section.body}
              </p>
            </section>
          ))}
        </div>
        <a
          href={repository + "/issues"}
          className="mt-10 inline-flex items-center gap-2 text-sm"
        >
          {text.footer.issues}
          <ArrowUpRight className="size-4" />
        </a>
      </main>
      <Footer language={language} />
    </>
  );
}
