"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BellIcon,
  CalendarPlusIcon,
  ClipboardListIcon,
  LayoutDashboardIcon,
  LogOutIcon,
  UserRoundIcon,
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
  { href: "/provider", label: "Overview", icon: LayoutDashboardIcon, exact: true },
  { href: "/provider/availability", label: "Availability", icon: CalendarPlusIcon },
  { href: "/provider/bookings", label: "Bookings", icon: ClipboardListIcon },
  { href: "/provider/notifications", label: "Notifications", icon: BellIcon },
  { href: "/provider/profile", label: "Profile", icon: UserRoundIcon },
];

function initials(email: string | null): string {
  if (!email) return "P";
  const parts = email.split("@")[0].split(/[._-]/);
  return parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? "").join("") || "P";
}

export default function ProviderLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  const { user, logout } = useSession();
  const { count: unreadCount } = useUnreadCount();

  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      {/* Sidebar */}
      <aside className="hidden w-60 shrink-0 flex-col border-r bg-primary text-primary-foreground lg:flex">
        <div className="flex h-14 items-center border-b border-primary-foreground/10 px-4">
          <Logo href="/provider" className="text-primary-foreground [&_span.text-\[15px\]]:text-primary-foreground" />
        </div>
        <nav aria-label="Provider dashboard" className="flex flex-1 flex-col gap-0.5 p-3">
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
                    ? "bg-primary-foreground text-primary"
                    : "text-primary-foreground/70 hover:bg-primary-foreground/10 hover:text-primary-foreground",
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
        <div className="border-t border-primary-foreground/10 p-3">
          <p className="px-3 text-xs text-primary-foreground/60">Provider workspace</p>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar */}
        <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b bg-primary px-4 text-primary-foreground sm:px-6">
          <div className="flex items-center gap-2 lg:hidden">
            <Logo href="/provider" className="text-primary-foreground [&_span.text-\[15px\]]:text-primary-foreground" />
          </div>
          <div className="hidden text-sm text-primary-foreground/70 lg:block">
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
              <Avatar className="border border-primary-foreground/20">
                <AvatarFallback>{initials(user?.email ?? null)}</AvatarFallback>
              </Avatar>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel>
                <p className="truncate text-sm font-medium">
                  {user?.email ?? "Signed in"}
                </p>
                <p className="text-xs font-normal text-muted-foreground">Provider</p>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onClick={() => void logout()}>
                <LogOutIcon aria-hidden="true" />
                Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </header>

        <main className="flex-1 px-4 py-6 sm:px-6">{children}</main>

        {/* Mobile bottom nav */}
        <nav
          aria-label="Provider dashboard"
          className="sticky bottom-0 z-30 flex h-16 items-stretch border-t bg-background lg:hidden"
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
