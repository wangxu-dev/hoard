import type { MetadataRoute } from "next";
import { siteUrl } from "@/lib/site";

export const dynamic = "force-static";
export default function sitemap(): MetadataRoute.Sitemap {
  if (!siteUrl) return [];
  return ["/", "/privacy/", "/en/", "/en/privacy/"].map((path) => ({
    url: new URL(path, siteUrl).href,
    alternates: {
      languages: {
        "zh-CN": new URL(path.includes("privacy") ? "/privacy/" : "/", siteUrl)
          .href,
        en: new URL(path.includes("privacy") ? "/en/privacy/" : "/en/", siteUrl)
          .href,
      },
    },
  }));
}
