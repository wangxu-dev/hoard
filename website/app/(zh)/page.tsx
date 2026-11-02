import { Home } from "@/components/home";
import { pageMetadata } from "@/lib/site";

export const metadata = pageMetadata("zh-CN");
export default function Page() {
  return <Home language="zh-CN" />;
}
