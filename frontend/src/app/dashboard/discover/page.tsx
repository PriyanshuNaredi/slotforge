import type { Metadata } from "next";
import { Suspense } from "react";

import { DiscoverContent } from "./discover-content";
import { Skeleton } from "@/components/ui/skeleton";

export const metadata: Metadata = {
  title: "Discover availability",
};

export default function DiscoverPage() {
  return (
    <Suspense fallback={<Skeleton className="mx-auto h-96 max-w-5xl rounded-xl" />}>
      <DiscoverContent />
    </Suspense>
  );
}
