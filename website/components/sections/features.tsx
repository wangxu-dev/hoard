import { Compass, CalendarRange } from "lucide-react";
import { DemoFrame, ExploreCaption } from "@/components/demo-frame";
import { LibraryPreview } from "@/components/library-preview";
import { Reveal } from "@/components/reveal";
import { dictionaries, type Language } from "@/lib/content";

export function Features({ language }: { language: Language }) {
  const { features, exploration } = dictionaries[language];
  const points = [
    { title: features.searchTitle, body: features.searchBody },
    { title: features.timelineTitle, body: features.timelineBody },
    { title: features.accountsTitle, body: features.accountsBody },
    { title: features.localTitle, body: features.localBody },
  ];
  return <>
    <section className="py-8">
      <LibraryPreview language={language} />
      <div className="site-container grid gap-8 py-12 md:grid-cols-2 lg:grid-cols-4">
        {points.map(point => <Reveal key={point.title}>
          <h3 className="text-base font-medium">{point.title}</h3>
          <p className="mt-3 text-sm leading-relaxed text-muted-foreground">{point.body}</p>
        </Reveal>)}
      </div>
    </section>
    <section id="exploration" className="site-container section-space">
      <Reveal className="section-heading">
        <p className="mb-4 text-sm text-muted-foreground">{exploration.eyebrow}</p>
        <h2>{exploration.title}</h2>
        <p>{exploration.description}</p>
      </Reveal>
      <Reveal>
        <figure>
          <DemoFrame language={language} exploration className="aspect-video min-h-128 w-full rounded-xl border border-border bg-background" />
          <ExploreCaption language={language} />
        </figure>
      </Reveal>
      <div className="mt-12 grid gap-8 md:grid-cols-2">
        {exploration.items.map((point, index) => {
          const Icon = [Compass, CalendarRange][index];
          return <Reveal key={point.title}>
            <Icon className="mb-4 size-5 text-muted-foreground" aria-hidden="true" />
            <h3 className="text-base font-medium">{point.title}</h3>
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">{point.body}</p>
          </Reveal>;
        })}
      </div>
    </section>
  </>;
}
