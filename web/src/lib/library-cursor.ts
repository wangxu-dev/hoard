import { getLibrarySource, type LibraryEntry, type ListQuery } from "./library";

const PAGE = 60;

/** ListCursor reads any position of one filtered and sorted list, loading pages on demand. */
export class ListCursor {
  private items: (LibraryEntry | undefined)[];
  private pages = new Map<number, Promise<void>>();
  total: number;
  constructor(readonly query: Omit<ListQuery, "offset" | "limit">, seed: (LibraryEntry | undefined)[], total: number) {
    this.items = [...seed];
    this.total = total;
  }
  peek(index: number): LibraryEntry | undefined { return this.items[index]; }
  indexOf(entry: LibraryEntry): number { return this.items.findIndex((item) => item?.id === entry.id && item.kind === entry.kind); }
  async at(index: number): Promise<LibraryEntry | undefined> {
    if (index < 0 || index >= this.total) return undefined;
    if (!this.items[index]) await this.load(Math.floor(index / PAGE) * PAGE);
    return this.items[index];
  }
  private load(offset: number): Promise<void> {
    let page = this.pages.get(offset);
    if (!page) {
      page = getLibrarySource().list({ ...this.query, offset, limit: PAGE }).then((result) => {
        result.items.forEach((item, i) => { this.items[offset + i] = item; });
        this.total = result.total;
      }).finally(() => this.pages.delete(offset));
      this.pages.set(offset, page);
    }
    return page;
  }
}

const warmed = new Set<string>();
/** warmImage fetches and decodes an image ahead of time so it shows at once. */
export function warmImage(url: string | undefined) {
  if (!url || warmed.has(url)) return;
  warmed.add(url);
  if (warmed.size > 400) warmed.delete(warmed.values().next().value as string);
  const image = new Image();
  image.decoding = "async";
  image.src = url;
  void image.decode().catch(() => warmed.delete(url));
}
