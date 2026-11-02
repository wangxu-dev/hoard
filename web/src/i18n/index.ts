import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import { App } from "@mygo";
import commonZh from "./locales/zh-CN/common.json";
import libraryZh from "./locales/zh-CN/library.json";
import settingsZh from "./locales/zh-CN/settings.json";
import commonEn from "./locales/en/common.json";
import libraryEn from "./locales/en/library.json";
import settingsEn from "./locales/en/settings.json";

const resources = {
  "zh-CN": { common: commonZh, library: libraryZh, settings: settingsZh },
  en: { common: commonEn, library: libraryEn, settings: settingsEn },
};

// The language lives in the library database, so a stale page storage value
// can no longer switch the interface. Chinese is the default.
localStorage.removeItem("hoard-lang");
void i18n.use(initReactI18next).init({
  resources,
  lng: "zh-CN",
  fallbackLng: "zh-CN",
  defaultNS: "common",
  interpolation: { escapeValue: false },
});

i18n.on("languageChanged", (nextLanguage) => {
  document.documentElement.lang = nextLanguage;
});
document.documentElement.lang = "zh-CN";
void App.language().then((saved) => {
  if (saved && saved !== i18n.language) void i18n.changeLanguage(saved);
}).catch(() => console.debug("[hoard:ui] language read failed"));

export function setLanguage(next: string) {
  void i18n.changeLanguage(next);
  void App.setLanguage(next).catch(() => console.debug("[hoard:ui] language save failed"));
}

export default i18n;
