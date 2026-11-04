import type { Metadata } from "next";

import { NotificationsContent } from "@/components/notifications/notifications-content";

export const metadata: Metadata = {
  title: "Notifications",
};

export default function ProviderNotificationsPage() {
  return (
    <NotificationsContent
      title="Notifications"
      description="New bookings and customer activity on your slots."
    />
  );
}
