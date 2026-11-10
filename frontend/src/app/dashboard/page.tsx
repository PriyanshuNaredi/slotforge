"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import {
  ArrowRightIcon,
  CalendarClockIcon,
  CalendarSearchIcon,
  CalendarX2Icon,
} from "lucide-react";

import { getAvailableSlot, listCustomerBookings } from "@/lib/api/endpoints";
import type { AvailableSlot } from "@/lib/api/types";
import { formatSlotRange, formatTime, timeZoneAbbr } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
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

  const confirmedBookings = useMemo(
    () => overview.data?.confirmed ?? [],
    [overview.data],
  );

  // Hydrate the fetched confirmed bookings' slots once. Slots expire from
  // the public endpoint, so a "confirmed" booking may be a past appointment
  // whose slot no longer resolves — those stay out of the dashboard.
  const [slots, setSlots] = useState<Record<string, AvailableSlot>>({});
  const [slotsLoaded, setSlotsLoaded] = useState(false);
  const [now] = useState(() => Date.now());
  useEffect(() => {
    let cancelled = false;
    Promise.all(
      confirmedBookings.map(async (booking) => {
        try {
          return [booking.slot_id, await getAvailableSlot(booking.slot_id)] as const;
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
      setSlotsLoaded(true);
    });
    return () => {
      cancelled = true;
    };
  }, [confirmedBookings]);

  // Upcoming = confirmed bookings whose slot resolves and starts in the future.
  const upcoming = confirmedBookings.filter((booking) => {
    const slot = slots[booking.slot_id];
    return slot !== undefined && new Date(slot.starts_at).getTime() > now;
  });
  const next = upcoming[0]
    ? { booking: upcoming[0], slot: slots[upcoming[0].slot_id] }
    : null;
  const hydrated =
    slotsLoaded || (!overview.loading && confirmedBookings.length === 0);

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <PageHeader
        title="Your appointments"
        description="What's booked, what's ahead."
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
        <>
          {/* Stat strip — one ruled line, not a row of cards */}
          <dl className="flex flex-wrap items-baseline gap-x-10 gap-y-2 border-b border-rule pb-4">
            <div className="flex items-baseline gap-2">
              <dt className="text-sm text-ink-soft">Confirmed</dt>
              <dd className="font-mono text-xl font-semibold tabular-nums">
                {overview.loading ? "—" : (overview.data?.confirmedTotal ?? 0)}
              </dd>
            </div>
            <div className="flex items-baseline gap-2">
              <dt className="text-sm text-ink-soft">All bookings</dt>
              <dd className="font-mono text-xl font-semibold tabular-nums">
                {overview.loading ? "—" : (overview.data?.totalBookings ?? "—")}
              </dd>
            </div>
          </dl>

          {/* Next appointment */}
          <section aria-labelledby="next-heading">
            <h2 id="next-heading" className="text-[11px] font-semibold uppercase tracking-[0.14em] text-coral-strong">
              Next appointment
            </h2>
            <div className="mt-3">
              {!hydrated ? (
                <div className="rounded-lg border border-rule bg-card px-5 py-6">
                  <div className="space-y-2">
                    <Skeleton className="h-6 w-56" />
                    <Skeleton className="h-4 w-72" />
                    <Skeleton className="h-4 w-40" />
                  </div>
                </div>
              ) : next ? (
                <div className="rounded-lg border border-rule bg-card">
                  <div className="flex flex-wrap items-start justify-between gap-4 px-5 py-5">
                    <div className="min-w-0">
                      <p className="font-display text-2xl font-semibold tracking-tight">
                        {formatSlotRange(next.slot.starts_at, next.slot.ends_at, next.slot.timezone).split(" · ")[0]}
                      </p>
                      <p className="mt-1 font-mono text-sm tabular-nums text-ink-soft">
                        {formatTime(new Date(next.slot.starts_at), next.slot.timezone)} –{" "}
                        {formatTime(new Date(next.slot.ends_at), next.slot.timezone)} (
                        {timeZoneAbbr(next.slot.timezone, new Date(next.slot.starts_at))})
                      </p>
                      <p className="mt-2 flex items-center gap-2 text-sm font-medium">
                        <CalendarClockIcon className="size-4 text-coral-strong" aria-hidden="true" />
                        {next.slot.provider_name}
                      </p>
                    </div>
                    <div className="flex flex-col items-end gap-3">
                      <StatusBadge status={next.booking.status} />
                      <Button asChild variant="outline" size="sm">
                        <Link href={`/dashboard/bookings/${next.booking.id}`}>
                          View booking
                          <ArrowRightIcon aria-hidden="true" />
                        </Link>
                      </Button>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-rule bg-card/60 px-5 py-4">
                  <div>
                    <p className="text-sm font-semibold">Nothing scheduled</p>
                    <p className="mt-0.5 text-sm text-ink-soft">
                      Browse open availability and book your next appointment in two clicks.
                    </p>
                  </div>
                  <Button asChild size="sm" variant="outline">
                    <Link href="/dashboard/discover">Browse availability</Link>
                  </Button>
                </div>
              )}
            </div>
          </section>

          {/* Confirmed ahead */}
          {hydrated && upcoming.length > 0 ? (
            <section aria-labelledby="upcoming-heading">
              <div className="flex items-center justify-between">
                <h2 id="upcoming-heading" className="text-[11px] font-semibold uppercase tracking-[0.14em] text-coral-strong">
                  Confirmed ahead
                </h2>
                <Button asChild variant="ghost" size="sm">
                  <Link href="/dashboard/bookings">All bookings</Link>
                </Button>
              </div>
              <ul className="mt-3 divide-y rounded-lg border border-rule bg-card">
                {upcoming.slice(1, 5).map((booking) => {
                  const slot = slots[booking.slot_id];
                  return (
                    <li key={booking.id}>
                      <Link
                        href={`/dashboard/bookings/${booking.id}`}
                        className="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5 transition-colors hover:bg-accent/50 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-coral"
                      >
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium">{slot.provider_name}</p>
                          <p className="mt-0.5 font-mono text-xs tabular-nums text-ink-soft">
                            {formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)}
                          </p>
                        </div>
                        <StatusBadge status={booking.status} />
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}
