import type { Metadata } from "next";
import { dictionaries, homePath, type Language } from "./content";
export const repository = "https://github.com/wangxu-dev/hoard";
// Downloads follow the published release while the app prepares its next version.
export const version = "0.1.8";
const configuredUrl = process.env.NEXT_PUBLIC_SITE_URL;
export const siteUrl = configuredUrl ? new URL(configuredUrl) : undefined;
if (
  siteUrl &&
  (siteUrl.protocol !== "https:" ||
    siteUrl.pathname !== "/" ||
    siteUrl.search ||
    siteUrl.hash ||
    siteUrl.username ||
    siteUrl.password)
) {
  throw new Error(
    "NEXT_PUBLIC_SITE_URL must be an HTTPS origin without a path or credentials",
  );
}

export function pageMetadata(language: Language, privacy = false): Metadata {
  const text = dictionaries[language];
  const path = homePath(language) + (privacy ? "privacy/" : "");
  const title = privacy ? text.privacy.metadata : text.meta.title;
  return {
    title,
    description: privacy ? text.privacy.intro : text.meta.description,
    ...(siteUrl
      ? {
          metadataBase: siteUrl,
          alternates: {
            canonical: path,
            languages: {
              "zh-CN": "/" + (privacy ? "privacy/" : ""),
              en: "/en/" + (privacy ? "privacy/" : ""),
              "x-default": "/" + (privacy ? "privacy/" : ""),
            },
          },
          openGraph: {
            type: "website",
            siteName: "Hoard",
            title,
            description: text.meta.description,
            url: path,
            locale: language === "en" ? "en_US" : "zh_CN",
            images: [{ url: "/og.png", width: 1200, height: 630, alt: title }],
          },
          twitter: {
            card: "summary_large_image",
            title,
            description: text.meta.description,
            images: ["/og.png"],
          },
        }
      : {}),
    robots: siteUrl
      ? { index: true, follow: true }
      : { index: false, follow: false },
    icons: { icon: "/icon.png", apple: "/icon.png" },
  };
}

export function downloadUrl(
  platform: "darwin" | "windows" | "linux",
  arch: "arm64" | "amd64",
) {
  const filename =
    platform === "darwin"
      ? `hoard-${version}-${arch}.dmg`
      : platform === "windows"
        ? `hoard-setup-${version}-${arch}.exe`
        : `hoard_${version}_${arch}.deb`;
  return `${repository}/releases/download/v${version}/${filename}`;
}
