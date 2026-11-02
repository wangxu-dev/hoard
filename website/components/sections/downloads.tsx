import { Reveal } from "@/components/reveal";
import { Card } from "@app-ui/card";
import {
  Laptop,
  Monitor,
  HardDrive,
  ArrowDown,
  ArrowUpRight,
} from "lucide-react";
import { Button } from "@app-ui/button";
import { downloadUrl, repository, version } from "@/lib/site";
import { dictionaries, type Language } from "@/lib/content";

export function Downloads({ language }: { language: Language }) {
  const text = dictionaries[language];
  return (
    <section id="download" className="site-container section-space">
      <Reveal className="section-heading">
        <h2>{text.download.title}</h2>
        <p>{text.download.description}</p>
        <p className="mt-4 font-mono text-xs">
          {text.download.versionLabel} · v{version}
        </p>
      </Reveal>
      <div className="download-grid">
        {(
          [
            {
              platform: "darwin",
              label: text.download.mac,
              icon: Laptop,
              arches: [
                { id: "arm64", label: text.download.apple },
                { id: "amd64", label: text.download.intel },
              ],
            },
            {
              platform: "windows",
              label: text.download.windows,
              icon: Monitor,
              arches: [
                { id: "amd64", label: text.download.x64 },
                { id: "arm64", label: text.download.arm64 },
              ],
            },
            {
              platform: "linux",
              label: text.download.linux,
              icon: HardDrive,
              arches: [
                { id: "amd64", label: text.download.x64 },
                { id: "arm64", label: text.download.arm64 },
              ],
            },
          ] as const
        ).map((item) => (
          <Card
            key={item.platform}
            className="download-card gap-0 p-6 shadow-none"
          >
            <item.icon className="size-7" />
            <h3 className="mt-4 text-lg font-semibold">{item.label}</h3>
            <div className="mt-6 flex flex-col gap-2">
              {item.arches.map((arch) => (
                <Button key={arch.id} asChild variant="outline">
                  <a
                    href={downloadUrl(item.platform, arch.id)}
                    aria-label={`${text.nav.download} Hoard ${item.label} ${arch.label}`}
                    data-download={`${item.platform}-${arch.id}`}
                  >
                    {arch.label}
                    <ArrowDown className="ml-auto" />
                  </a>
                </Button>
              ))}
            </div>
          </Card>
        ))}
      </div>

      <div className="mt-7 flex flex-wrap justify-center gap-6 text-sm">
        <a
          href={`${repository}/releases/latest`}
          className="inline-flex items-center gap-1.5"
        >
          {text.download.all}
          <ArrowUpRight className="size-4" />
        </a>
      </div>
    </section>
  );
}
