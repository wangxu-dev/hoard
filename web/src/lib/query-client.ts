import { QueryClient } from "@tanstack/react-query";

/** One client for the whole app, so data outlives the page that asked for it. */
export const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, gcTime: 30 * 60_000, refetchOnWindowFocus: false, retry: false } },
});

// Grid lists hold thousands of slots and change a page at a time; comparing them deeply on every
// update would cost more than it saves.
queryClient.setQueryDefaults(["grid"], { structuralSharing: false });
