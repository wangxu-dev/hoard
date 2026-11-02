import { Privacy } from "@/components/privacy";
import { pageMetadata } from "@/lib/site";

export const metadata = pageMetadata("en", true);
export default function Page() {
  return <Privacy language="en" />;
}
