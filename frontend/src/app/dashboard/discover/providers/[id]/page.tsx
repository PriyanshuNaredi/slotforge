import type { Metadata } from "next";

import { ProviderProfileContent } from "@/components/customer/provider-profile";

export const metadata: Metadata = {
  title: "Provider profile",
};

export default function ProviderProfilePage() {
  return <ProviderProfileContent />;
}
