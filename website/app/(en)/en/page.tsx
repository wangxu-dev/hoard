import { Home } from "@/components/home";
import { pageMetadata } from "@/lib/site";

export const metadata = pageMetadata("en");
export default function Page() {
  return <Home language="en" />;
}
