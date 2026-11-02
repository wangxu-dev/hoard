import "../globals.css";
import { SiteLayout } from "@/components/shell";

export default function Layout({ children }: { children: React.ReactNode }) {
  return <SiteLayout language="zh-CN">{children}</SiteLayout>;
}
