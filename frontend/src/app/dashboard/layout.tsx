"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  CalendarSearchIcon,
  CalendarClockIcon,
  CalendarDaysIcon,
  BellIcon,
  LayoutDashboardIcon,
  LogOutIcon,
} from "lucide-react";

import { Logo } from "@/components/shared/logo";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useSession } from "@/components/providers/session-provider";
import { UnreadBadge } from "@/components/shared/unread-badge";
import { useUnreadCount } from "@/hooks/use-unread-count";
import { cn } from "@/lib/utils";

const nav = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboardIcon, exact: true },
  { href: "/dashboard/discover", label: "Discover", icon: CalendarSearchIcon },
  { href: "/dashboard/bookings", label: "My bookings", icon: CalendarClockIcon },
  { href: "/dashboard/calendar", label: "Calendar", icon: CalendarDaysIcon },
  { href: "/dashboard/notifications", label: "Notifications", icon: BellIcon },
];

function initials(email: string | null): string {
  if (!email) return "U";
  const parts = email.split("@")[0].split(/[._-]/);
  return parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? "").join("") || "U";
}

export default function DashboardLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  const { user, logout } = useSession();
  const { count: unreadCount } = useUnreadCount();

  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      {/* Sidebar */}
      <aside className="hidden w-60 shrink-0 flex-col border-r border-cream/15 bg-navy text-cream lg:flex">
        <div className="flex h-14 items-center border-b px-4">
          <Logo href="/dashboard" />
        </div>
        <nav aria-label="Dashboard" className="flex flex-1 flex-col gap-0.5 p-3">
          {nav.map((item) => {
            const active = item.exact
              ? pathname === item.href
              : pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "bg-coral text-navy"
                    : "text-cream/70 hover:bg-white/10 hover:text-cream",
                )}
              >
                <item.icon className="size-4" aria-hidden="true" />
                {item.label}
                {item.href.endsWith("notifications") ? (
                  <UnreadBadge count={unreadCount} className="ml-auto" />
                ) : null}
              </Link>
            );
          })}
        </nav>
        <div className="border-t border-cream/15 p-3">
          <p className="px-3 text-xs text-cream/60">Customer account</p>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar */}
        <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b bg-background/85 px-4 backdrop-blur sm:px-6">
          <div className="flex items-center gap-2 lg:hidden">
            <Logo href="/dashboard" />
          </div>
          <div className="hidden text-sm text-muted-foreground lg:block">
            {nav.find((item) =>
              item.exact
                ? pathname === item.href
                : pathname.startsWith(item.href),
            )?.label ?? "Overview"}
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger
              className="rounded-full outline-none focus-visible:ring-[3px] focus-visible:ring-ring/40 cursor-pointer"
              aria-label="Account menu"
            >
              <Avatar>
                <AvatarFallback>{initials(user?.email ?? null)}</AvatarFallback>
              </Avatar>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel>
                <p className="truncate text-sm font-medium">
                  {user?.email ?? "Signed in"}
                </p>
                <p className="text-xs font-normal text-muted-foreground">
                  Customer
                </p>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onClick={() => void logout()}>
                <LogOutIcon aria-hidden="true" />
                Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </header>

        <main className="flex-1 px-4 py-6 pb-24 sm:px-6 lg:pb-8">{children}</main>

        {/* Mobile bottom nav */}
        <nav
          aria-label="Dashboard"
          className="fixed inset-x-0 bottom-0 z-30 flex h-16 items-stretch border-t bg-background/95 backdrop-blur lg:hidden"
        >
          {nav.map((item) => {
            const active = item.exact
              ? pathname === item.href
              : pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium",
                  active ? "text-brand-strong" : "text-muted-foreground",
                )}
              >
                <item.icon className="size-5" aria-hidden="true" />
                {item.label}
                {item.href.endsWith("notifications") && unreadCount > 0 ? (
                  <span
                    className="absolute right-3 top-2 size-2 rounded-full bg-brand"
                    aria-hidden="true"
                  />
                ) : null}
              </Link>
            );
          })}
        </nav>
      </div>
    </div>
  );
}
