import type { LibraryEntry } from "@/lib/library";
export interface ContentWork {
  publishedAt?: number;
  outsideTimeRange?: boolean;
  key: string;
  id: string;
  sourceCode: string;
  kind: "video" | "gallery";
  title: string;
  source: string;
  author: string;
  avatar: string;
  cover: string;
  topics: { id: string; label: string }[];
}
export interface PanelView {
  key: string;
  label: string;
  work: ContentWork | null;
  works: ContentWork[];
  back: () => void;
  openWork: (key: string) => void;
  focusTheme: (id: string) => void;
  imageStatus: (url: string, state: number) => void;
}

// Duration and image count remain absent from the exploration snapshot.
export function libraryEntry(work: ContentWork): LibraryEntry {
  return {source:work.sourceCode,id:work.id,kind:work.kind,title:work.title,author:work.author,authorAvatar:work.avatar,cover:work.cover,publishedAt:work.publishedAt || 0,durationMs:0,imageCount:0};
}
