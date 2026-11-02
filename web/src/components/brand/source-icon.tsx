import { siTiktok } from "simple-icons";
import { Globe } from "lucide-react";
export function SourceIcon({ source, className = "size-3.5" }: { source: string; className?: string }) {
  if (source !== "douyin") return <Globe className={className} aria-hidden="true" />;
  return <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden="true"><path d={siTiktok.path} /></svg>;
}
