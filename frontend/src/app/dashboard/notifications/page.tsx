import type { Metadata } from "next";

import { NotificationsContent } from "@/components/notifications/notifications-content";

export const metadata: Metadata = {
  title: "Notifications",
};

export default function NotificationsPage() {
  return (
    <NotificationsContent
      title="Notifications"
      description="Booking updates and activity on your appointments."
    />
  );
}
