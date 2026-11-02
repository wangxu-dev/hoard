import { Header, Footer } from "@/components/shell";
import { Hero } from "@/components/sections/hero";
import { Sources } from "@/components/sections/sources";
import { Features } from "@/components/sections/features";
import { Downloads } from "@/components/sections/downloads";
import { Questions } from "@/components/sections/questions";
import { dictionaries, type Language } from "@/lib/content";
import { repository, siteUrl, version } from "@/lib/site";

export function Home({ language }: { language: Language }) {
  const text = dictionaries[language];
  const structuredData = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "Hoard",
    applicationCategory: "MultimediaApplication",
    operatingSystem: "macOS, Windows, Linux",
    softwareVersion: version,
    description: text.meta.description,
    license: repository + "/blob/main/LICENSE",
    downloadUrl: repository + "/releases/latest",
    offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
    ...(siteUrl
      ? { url: new URL(language === "en" ? "/en/" : "/", siteUrl).href }
      : {}),
  };
  return (
    <>
      <Header language={language} />
      <main id="main">
        <Hero language={language} />
        <Sources language={language} />
        <Features language={language} />
        <Downloads language={language} />
        <Questions language={language} />
      </main>
      <Footer language={language} />
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify(structuredData).replace(/</g, "\\u003c"),
        }}
      />
    </>
  );
}
