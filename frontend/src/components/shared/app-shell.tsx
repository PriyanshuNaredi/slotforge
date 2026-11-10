"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogOutIcon } from "lucide-react";

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

export interface AppNavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  exact?: boolean;
}

interface AppShellProps {
  nav: AppNavItem[];
  /** aria-label for both nav landmarks. */
  navLabel: string;
  /** Sidebar footer role anchor, e.g. "Customer account". */
  roleLabel: string;
  /** Role name shown in the account menu. */
  roleName: string;
  homeHref: string;
  children: React.ReactNode;
}

function initials(email: string | null, fallback: string): string {
  if (!email) return fallback;
  const parts = email.split("@")[0].split(/[._-]/);
  return (
    parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? "").join("") || fallback
  );
}

/**
 * The one shell both sides share: navy sidebar on desktop, paper top bar +
 * bottom nav on mobile. Active nav rows are paper-on-navy — the selected
 * line of the board.
 */
export function AppShell({
  nav,
  navLabel,
  roleLabel,
  roleName,
  homeHref,
  children,
}: AppShellProps) {
  const pathname = usePathname();
  const { user, logout } = useSession();
  const { count: unreadCount } = useUnreadCount();

  const accountMenu = (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="rounded-full outline-none cursor-pointer focus-visible:ring-[3px] focus-visible:ring-coral/50"
        aria-label="Account menu"
      >
        <Avatar className="border border-cream/25">
          <AvatarFallback>{initials(user?.email ?? null, roleName[0])}</AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel>
          <p className="truncate text-sm font-medium">{user?.email ?? "Signed in"}</p>
          <p className="text-xs font-normal text-muted-foreground">{roleName}</p>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={() => void logout()}>
          <LogOutIcon aria-hidden="true" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      {/* Sidebar (desktop) */}
      <aside className="hidden w-60 shrink-0 flex-col border-r border-cream/15 bg-navy text-cream lg:flex">
        <div className="flex h-14 items-center border-b border-cream/15 px-4">
          <Logo href={homeHref} tone="inverse" />
        </div>
        <nav aria-label={navLabel} className="flex flex-1 flex-col gap-0.5 p-3">
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
                  "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-coral/70 focus-visible:ring-inset focus-visible:outline-none",
                  active
                    ? "bg-background text-ink"
                    : "text-cream/70 hover:bg-white/10 hover:text-cream",
                )}
              >
                <item.icon
                  className={cn("size-4", active && "text-coral-strong")}
                  aria-hidden="true"
                />
                {item.label}
                {item.href.endsWith("notifications") ? (
                  <UnreadBadge count={unreadCount} className="ml-auto" />
                ) : null}
              </Link>
            );
          })}
        </nav>
        <div className="flex items-center justify-between border-t border-cream/15 p-3">
          <p className="px-1 text-xs text-cream/60">{roleLabel}</p>
          {accountMenu}
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar (mobile only) */}
        <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b bg-background px-4 sm:px-6 lg:hidden">
          <Logo href={homeHref} />
          {accountMenu}
        </header>

        <main className="flex-1 px-4 py-6 pb-24 sm:px-6 lg:pb-10">{children}</main>

        {/* Bottom nav (mobile); safe-area padding keeps it above the
            iOS home indicator. */}
        <nav
          aria-label={navLabel}
          className="sticky bottom-0 z-30 flex h-16 items-stretch border-t bg-background pb-[env(safe-area-inset-bottom)] lg:hidden"
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
                  "relative flex flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-coral/70",
                  active ? "text-ink" : "text-ink-soft",
                )}
              >
                <item.icon
                  className={cn("size-5", active && "text-coral-strong")}
                  aria-hidden="true"
                />
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
