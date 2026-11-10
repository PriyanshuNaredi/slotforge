"use client";

import {
  BellIcon,
  CalendarDaysIcon,
  CalendarPlusIcon,
  ClipboardListIcon,
  LayoutDashboardIcon,
  UserRoundIcon,
} from "lucide-react";

import { AppShell } from "@/components/shared/app-shell";

const nav = [
  { href: "/provider", label: "Overview", icon: LayoutDashboardIcon, exact: true },
  { href: "/provider/availability", label: "Availability", icon: CalendarPlusIcon },
  { href: "/provider/bookings", label: "Bookings", icon: ClipboardListIcon },
  { href: "/provider/calendar", label: "Calendar", icon: CalendarDaysIcon },
  { href: "/provider/notifications", label: "Notifications", icon: BellIcon },
  { href: "/provider/profile", label: "Profile", icon: UserRoundIcon },
];

export default function ProviderLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <AppShell
      nav={nav}
      navLabel="Provider dashboard"
      roleLabel="Provider workspace"
      roleName="Provider"
      homeHref="/provider"
    >
      {children}
    </AppShell>
  );
}
