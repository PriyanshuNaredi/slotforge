import type { Metadata } from "next";
import { Suspense } from "react";

import { CustomerBookingsContent } from "./bookings-content";
import { Skeleton } from "@/components/ui/skeleton";

export const metadata: Metadata = {
  title: "My bookings",
};

export default function CustomerBookingsPage() {
  return (
    <Suspense fallback={<Skeleton className="mx-auto h-96 max-w-4xl rounded-xl" />}>
      <CustomerBookingsContent />
    </Suspense>
  );
}
