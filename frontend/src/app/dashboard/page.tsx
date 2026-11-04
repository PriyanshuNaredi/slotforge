"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ArrowRightIcon,
  CalendarClockIcon,
  CalendarSearchIcon,
  CalendarX2Icon,
  UserIcon,
} from "lucide-react";

import { getAvailableSlot, listCustomerBookings } from "@/lib/api/endpoints";
import type { AvailableSlot, Booking } from "@/lib/api/types";
import { formatRelative, formatSlotRange } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/use-async-data";

export default function CustomerOverviewPage() {
  const overview = useAsyncData("overview", () =>
    Promise.all([
      listCustomerBookings({ status: "confirmed", limit: 5 }),
      listCustomerBookings({ limit: 1 }),
    ]).then(([confirmed, all]) => ({
      confirmed: confirmed.bookings,
      // total counts every confirmed booking, not just the fetched page.
      confirmedTotal: confirmed.total,
      totalBookings: all.total,
    })),
  );

  const upcoming = overview.data?.confirmed ?? [];

  // Hydrate the soonest upcoming booking with its slot details.
  const soonest = upcoming[0];
  const [next, setNext] = useState<{ booking: Booking; slot: AvailableSlot | null } | null>(null);
  // Only show hydration results for the booking currently in front.
  const nextIsCurrent = next?.booking.id === soonest?.id;

  useEffect(() => {
    if (!soonest) return;
    let cancelled = false;
    getAvailableSlot(soonest.slot_id).then(
      (slot) => {
        if (!cancelled) setNext({ booking: soonest, slot });
      },
      () => {
        if (!cancelled) setNext({ booking: soonest, slot: null });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [soonest]);

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageHeader
        title="Overview"
        description="Your appointments at a glance."
        actions={
          <Button asChild size="sm">
            <Link href="/dashboard/discover">
              <CalendarSearchIcon aria-hidden="true" />
              Find a slot
            </Link>
          </Button>
        }
      />

      {overview.error ? (
        <ErrorState
          icon={CalendarX2Icon}
          title="Couldn't load your dashboard"
          description={overview.error}
          action={
            <Button variant="outline" size="sm" onClick={overview.reload}>
              Retry
            </Button>
          }
        />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
          {/* Next appointment */}
          <Card>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold text-muted-foreground">
                  Next appointment
                </h2>
                {nextIsCurrent && next ? <StatusBadge status={next.booking.status} /> : null}
              </div>

              {overview.loading && !nextIsCurrent ? (
                <div className="space-y-2">
                  <Skeleton className="h-5 w-48" />
                  <Skeleton className="h-4 w-64" />
                  <Skeleton className="h-4 w-40" />
                </div>
              ) : nextIsCurrent && next ? (
                <>
                  <div>
                    <p className="flex items-center gap-2 text-lg font-semibold">
                      <UserIcon className="size-4 text-muted-foreground" aria-hidden="true" />
                      {next.slot?.provider_name ?? "Provider"}
                    </p>
                    {next.slot ? (
                      <p className="mt-1 text-sm text-muted-foreground">
                        {formatSlotRange(
                          next.slot.starts_at,
                          next.slot.ends_at,
                          next.slot.timezone,
                        )}
                      </p>
                    ) : (
                      <p className="mt-1 text-sm text-muted-foreground">
                        Slot details are no longer available for this booking.
                      </p>
                    )}
                  </div>
                  <Button asChild variant="outline" size="sm">
                    <Link href={`/dashboard/bookings/${next.booking.id}`}>
                      View booking
                      <ArrowRightIcon aria-hidden="true" />
                    </Link>
                  </Button>
                </>
              ) : (
                <EmptyState
                  className="border-0 bg-transparent py-8"
                  icon={CalendarClockIcon}
                  title="Nothing scheduled"
                  description="You have no upcoming appointments. Browse open availability to book one."
                  action={
                    <Button asChild variant="outline" size="sm">
                      <Link href="/dashboard/discover">Browse availability</Link>
                    </Button>
                  }
                />
              )}
            </CardContent>
          </Card>

          {/* Stats */}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
            <Card>
              <CardContent>
                <p className="text-sm text-muted-foreground">Upcoming</p>
                {overview.loading ? (
                  <Skeleton className="mt-2 h-8 w-12" />
                ) : (
                  <p className="mt-1 text-3xl font-semibold tracking-tight">
                    {overview.data?.confirmedTotal ?? 0}
                  </p>
                )}
                <p className="mt-1 text-xs text-muted-foreground">confirmed appointments</p>
              </CardContent>
            </Card>
            <Card>
              <CardContent>
                <p className="text-sm text-muted-foreground">All bookings</p>
                {overview.loading ? (
                  <Skeleton className="mt-2 h-8 w-12" />
                ) : (
                  <p className="mt-1 text-3xl font-semibold tracking-tight">
                    {overview.data?.totalBookings ?? "—"}
                  </p>
                )}
                <p className="mt-1 text-xs text-muted-foreground">across every status</p>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* Upcoming list */}
      {!overview.loading && upcoming.length > 0 ? (
        <section aria-labelledby="upcoming-heading">
          <div className="flex items-center justify-between">
            <h2 id="upcoming-heading" className="text-sm font-semibold text-muted-foreground">
              Your upcoming schedule
            </h2>
            <Button asChild variant="ghost" size="sm">
              <Link href="/dashboard/bookings">All bookings</Link>
            </Button>
          </div>
          <UpcomingList bookings={upcoming.slice(0, 5)} />
        </section>
      ) : null}
    </div>
  );
}

function UpcomingList({ bookings }: { bookings: Booking[] }) {
  const [slots, setSlots] = useState<Record<string, AvailableSlot>>({});

  useEffect(() => {
    let cancelled = false;
    Promise.all(
      bookings.map(async (booking) => {
        try {
          const slot = await getAvailableSlot(booking.slot_id);
          return [booking.id, slot] as const;
        } catch {
          return null;
        }
      }),
    ).then((entries) => {
      if (cancelled) return;
      const map: Record<string, AvailableSlot> = {};
      for (const entry of entries) {
        if (entry) map[entry[0]] = entry[1];
      }
      setSlots(map);
    });
    return () => {
      cancelled = true;
    };
  }, [bookings]);

  return (
    <ul className="mt-3 divide-y rounded-xl border bg-card">
      {bookings.map((booking) => {
        const slot = slots[booking.id];
        return (
          <li key={booking.id}>
            <Link
              href={`/dashboard/bookings/${booking.id}`}
              className="flex flex-wrap items-center justify-between gap-3 px-4 py-3 transition-colors hover:bg-accent/40 sm:px-5"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">
                  {slot?.provider_name ?? "Provider"}
                </p>
                <p className="text-xs text-muted-foreground">
                  {slot
                    ? formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)
                    : `Booked ${formatRelative(new Date(booking.created_at))}`}
                </p>
              </div>
              <StatusBadge status={booking.status} />
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
