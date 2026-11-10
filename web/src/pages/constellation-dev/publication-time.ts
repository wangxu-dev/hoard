export type MonthRange = [number, number];
export interface TimeSelection { range: MonthRange | null; includeUnknown: boolean }
export interface DatedWork { publishedAt?: number; monthKeys?: { source_time?: string | null } }
export interface MonthBin { month: number; count: number }

export function workMonth(work: DatedWork): number | null {
  const key = work.monthKeys?.source_time;
  if (!key || !/^\d{4}-(0[1-9]|1[0-2])$/.test(key)) return null;
  return Number(key.slice(0, 4)) * 12 + Number(key.slice(5)) - 1;
}
export function publicationBins(works: DatedWork[]) {
  const counts = new Map<number, number>();
  let unknown = 0;
  for (const work of works) {
    const month = workMonth(work);
    if (month === null) unknown++;
    else counts.set(month, (counts.get(month) || 0) + 1);
  }
  const keys = [...counts.keys()];
  if (!keys.length) return { bins: [] as MonthBin[], unknown };
  const first = Math.min(...keys), last = Math.max(...keys);
  return { bins: Array.from({ length: last - first + 1 }, (_, i) => ({ month: first + i, count: counts.get(first + i) || 0 })), unknown };
}
export function matchesPublication(work: DatedWork, selection: TimeSelection): boolean {
  if (!selection.range) return true;
  const month = workMonth(work);
  return month === null ? selection.includeUnknown : month >= selection.range[0] && month <= selection.range[1];
}
export function clampRange(range: MonthRange, first: number, last: number): MonthRange {
  const width = Math.min(last - first, Math.max(0, range[1] - range[0]));
  const start = Math.max(first, Math.min(last - width, range[0]));
  return [start, start + width];
}
export function nextPlaybackRange(range: MonthRange | null, first: number, last: number): MonthRange | null {
  if (!range || range[0] === first && range[1] === last || range[1] >= last) return [first, first];
  return clampRange([range[0] + 1, range[1] + 1], first, last);
}
export function monthDate(month: number): Date {
  return new Date(Math.floor(month / 12), month % 12, 1);
}
