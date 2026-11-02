import { Privacy } from "@/components/privacy";
import { pageMetadata } from "@/lib/site";

export const metadata = pageMetadata("zh-CN", true);
export default function Page() {
  return <Privacy language="zh-CN" />;
}
