import Image from "next/image";
import { GithubIcon } from "@/components/github-icon";
import { Button } from "@app-ui/button";
import { Preferences } from "@/components/preferences";
import { Providers } from "@/components/providers";
import { dictionaries, homePath, type Language } from "@/lib/content";
import { repository } from "@/lib/site";

export function Brand({ language }: { language: Language }) {
  return (
    <a
      href={homePath(language)}
      aria-label="Hoard"
      className="brand flex items-center gap-2.5"
    >
      <Image src="/icon.png" width={28} height={28} alt="" />
      <span className="text-sm font-semibold tracking-tight">Hoard</span>
    </a>
  );
}

export function SiteLayout({
  language,
  children,
}: {
  language: Language;
  children: React.ReactNode;
}) {
  return (
    <html lang={language} suppressHydrationWarning>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}

export function Header({
  language,
  privacy = false,
}: {
  language: Language;
  privacy?: boolean;
}) {
  const text = dictionaries[language];
  return (
    <>
      <a className="skip-link" href="#main">
        {text.nav.skip}
      </a>
      <header className="site-header">
        <div className="site-container flex h-12 items-center justify-between gap-4">
          <Brand language={language} />
          <nav
            aria-label={text.nav.sections}
            className="hidden items-center gap-1 sm:flex"
          >
            {[
              { id: "preview", label: text.nav.preview },
              { id: "download", label: text.nav.download },
              { id: "faq", label: text.nav.faq },
            ].map((item) => (
              <Button key={item.id} asChild variant="ghost" size="sm">
                <a href={`${privacy ? homePath(language) : ""}#${item.id}`}>
                  {item.label}
                </a>
              </Button>
            ))}
          </nav>
          <div className="flex items-center gap-2">
            <Button asChild variant="ghost" size="icon-sm">
              <a href={repository} aria-label="GitHub">
                <GithubIcon />
              </a>
            </Button>
            <Preferences language={language} privacy={privacy} />
          </div>
        </div>
      </header>
    </>
  );
}

export function Footer({ language }: { language: Language }) {
  const text = dictionaries[language];
  return (
    <footer className="site-container border-t py-10">
      <div className="flex flex-col justify-between gap-6 sm:flex-row sm:items-center">
        <div>
          <Brand language={language} />
          <p className="mt-3 text-sm text-muted-foreground">
            {text.footer.tagline}
          </p>
        </div>
        <nav className="flex flex-wrap gap-x-6 gap-y-3 text-sm text-muted-foreground">
          <a href={repository} aria-label="GitHub">
            <GithubIcon />
          </a>
          <a href={`${repository}/issues`}>{text.footer.issues}</a>
          <a href={`${homePath(language)}privacy/`}>{text.footer.privacy}</a>
          <a href={`${repository}/blob/main/LICENSE`}>{text.footer.license}</a>
        </nav>
      </div>
    </footer>
  );
}
