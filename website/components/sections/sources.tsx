import {
  siBilibili,
  siTiktok,
  siXiaohongshu,
  siYoutube,
  siInstagram,
  siPinterest,
  siVimeo,
} from "simple-icons";
import { Marquee } from "@/components/ui/marquee";
import { dictionaries, type Language } from "@/lib/content";

export function Sources({ language }: { language: Language }) {
  const text = dictionaries[language].sources;
  const platforms = [
    { name: text.douyin, icon: siTiktok, supported: true },
    { name: text.bilibili, icon: siBilibili, supported: true },
    { name: text.xiaohongshu, icon: siXiaohongshu },
    { name: "YouTube", icon: siYoutube },
    { name: "Instagram", icon: siInstagram },
    { name: "Pinterest", icon: siPinterest },
    { name: "Vimeo", icon: siVimeo },
  ];
  return (
    <section id="preview" className="border-y bg-sidebar py-3">
      <div className="site-container flex min-w-0 items-center gap-8">
        <Marquee
          pauseOnHover
          repeat={2}
          className="min-w-0 flex-1 p-0 [--gap:2rem]"
        >
          {platforms.map((platform) => (
            <span
              key={platform.name}
              title={platform.supported ? text.connected : text.future}
              className={`inline-flex shrink-0 items-center gap-2 whitespace-nowrap text-sm ${platform.supported ? "text-foreground" : "text-muted-foreground"}`}
            >
              <svg
                viewBox="0 0 24 24"
                className="size-4 fill-current"
                aria-hidden="true"
              >
                <path d={platform.icon.path} />
              </svg>
              {platform.name}
            </span>
          ))}
        </Marquee>
        <p className="shrink-0 whitespace-nowrap text-xs text-muted-foreground">{text.statusNote}</p>
      </div>
    </section>
  );
}
