import zh from "../i18n/zh-CN.json";
import en from "../i18n/en.json";

export type Language = "zh-CN" | "en";
export const dictionaries = { "zh-CN": zh, en };
export const homePath = (language: Language) =>
  language === "en" ? "/en/" : "/";
