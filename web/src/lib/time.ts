const sameDay = (a: Date, b: Date) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();

/** formatWhen renders a Unix time as 14:05, 昨天 14:05, 10月28日 or 2025年10月28日 in the given language. */
export function formatWhen(seconds: number, language: string, now = new Date()): string {
  const date = new Date(seconds * 1000);
  const clock = new Intl.DateTimeFormat(language, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(date);
  if (sameDay(date, now)) return clock;
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  if (sameDay(date, yesterday)) {
    const word = new Intl.RelativeTimeFormat(language, { numeric: "auto" }).format(-1, "day");
    return `${word.charAt(0).toLocaleUpperCase(language)}${word.slice(1)} ${clock}`;
  }
  if (date.getFullYear() === now.getFullYear()) return new Intl.DateTimeFormat(language, { month: "long", day: "numeric" }).format(date);
  return new Intl.DateTimeFormat(language, { year: "numeric", month: "long", day: "numeric" }).format(date);
}
