"use client";

import {
  BellIcon,
  CalendarClockIcon,
  CalendarDaysIcon,
  CalendarSearchIcon,
  LayoutDashboardIcon,
} from "lucide-react";

import { AppShell } from "@/components/shared/app-shell";

const nav = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboardIcon, exact: true },
  { href: "/dashboard/discover", label: "Discover", icon: CalendarSearchIcon },
  { href: "/dashboard/bookings", label: "My bookings", icon: CalendarClockIcon },
  { href: "/dashboard/calendar", label: "Calendar", icon: CalendarDaysIcon },
  { href: "/dashboard/notifications", label: "Notifications", icon: BellIcon },
];

export default function DashboardLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <AppShell
      nav={nav}
      navLabel="Dashboard"
      roleLabel="Customer account"
      roleName="Customer"
      homeHref="/dashboard"
    >
      {children}
    </AppShell>
  );
}
