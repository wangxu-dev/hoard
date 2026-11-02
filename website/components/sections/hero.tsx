import { ArrowDown, ChevronDown } from "lucide-react";
import { GithubIcon } from "@/components/github-icon";
import { Button } from "@app-ui/button";
import { dictionaries, type Language } from "@/lib/content";
import { repository } from "@/lib/site";
import { Reveal } from "@/components/reveal";
import { CollectionArt } from "@/components/collection-art";

export function Hero({ language }: { language: Language }) {
  const text = dictionaries[language];
  return (
    <section className="hero site-container relative">
      <Reveal className="hero-copy mx-auto max-w-3xl text-center">
        <p className="eyebrow">
          <span className="status-dot" />
          {text.hero.eyebrow}
        </p>
        <h1 className="hero-title">
          {text.hero.line1}
          <br />
          <span className="text-brand">{text.hero.line2}</span>
        </h1>
        <p className="hero-description mx-auto max-w-xl text-muted-foreground">
          {text.hero.description}
        </p>
        <div className="mt-6 flex flex-wrap justify-center gap-3">
          <Button asChild size="lg">
            <a href="#download">
              <ArrowDown />
              {text.hero.download}
            </a>
          </Button>
          <Button asChild variant="outline" size="lg">
            <a href={repository}>
              <GithubIcon />
              {text.hero.github}
            </a>
          </Button>
        </div>
        <p className="mt-5 text-xs text-muted-foreground">{text.hero.note}</p>
      </Reveal>
      <CollectionArt language={language} />
      <a
        href="#preview"
        className="absolute bottom-6 flex flex-col items-center gap-3 text-xs text-muted-foreground"
      >
        {text.hero.caption}
        <ChevronDown className="size-4" aria-hidden="true" />
      </a>
    </section>
  );
}
