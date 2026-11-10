"use client";

import Link from "next/link";
import {
  ArrowRightIcon,
  CalendarPlusIcon,
  ClipboardListIcon,
  CalendarX2Icon,
  UserRoundPlusIcon,
} from "lucide-react";

import {
  getMyProviderProfile,
  listProviderBookings,
  listProviderSlots,
} from "@/lib/api/endpoints";
import type { ProviderBooking, ProviderProfile, Slot } from "@/lib/api/types";
import { ApiError } from "@/lib/api/client";
import { formatSlotRange } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/use-async-data";

interface ProviderHome {
  profile: ProviderProfile | null;
  slots: Slot[];
  upcoming: ProviderBooking[];
  /** Confirmed, in the future — full count, not just the fetched page. */
  upcomingTotal: number;
  totalBookings: number;
}

export default function ProviderOverviewPage() {
  const home = useAsyncData("provider-home", async (): Promise<ProviderHome> => {
    const [slotsResult, upcomingResult, totalResult] = await Promise.all([
      listProviderSlots(),
      listProviderBookings({
        status: "confirmed",
        from: new Date().toISOString(),
        limit: 5,
      }),
      listProviderBookings({ limit: 1 }),
    ]);

    let profile: ProviderProfile | null = null;
    try {
      profile = await getMyProviderProfile();
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 404)) throw err;
    }

    return {
      profile,
      slots: slotsResult.slots,
      upcoming: upcomingResult.bookings,
      upcomingTotal: upcomingResult.total,
      totalBookings: totalResult.total,
    };
  });

  const data = home.data;
  const openSlots = data?.slots.filter((slot) => slot.status === "open").length ?? 0;

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <PageHeader
        title="Provider overview"
        description={data?.profile ? `Signed in as ${data.profile.display_name}.` : "Your booking operations at a glance."}
        actions={
          <Button asChild size="sm">
            <Link href="/provider/availability">
              <CalendarPlusIcon aria-hidden="true" />
              New slot
            </Link>
          </Button>
        }
      />

      {/* Onboarding prompt when no profile exists yet */}
      {data && data.profile === null ? (
        <div className="flex flex-wrap items-center justify-between gap-4 rounded-md border border-coral/30 bg-coral-wash/50 px-5 py-4">
          <div className="flex items-start gap-3">
            <UserRoundPlusIcon className="mt-0.5 size-5 text-coral-strong" aria-hidden="true" />
            <div>
              <p className="text-sm font-semibold">Finish setting up your profile</p>
              <p className="mt-0.5 text-sm text-ink-soft">
                Customers can&apos;t see a provider without a display name and
                timezone. It takes one minute.
              </p>
            </div>
          </div>
          <Button asChild size="sm" variant="brand">
            <Link href="/provider/profile">
              Create profile
              <ArrowRightIcon aria-hidden="true" />
            </Link>
          </Button>
        </div>
      ) : null}

      {home.error ? (
        <ErrorState
          icon={CalendarX2Icon}
          title="Couldn't load your dashboard"
          description={home.error}
          action={
            <Button variant="outline" size="sm" onClick={home.reload}>
              Retry
            </Button>
          }
        />
      ) : (
        <>
          {/* Stat strip — one ruled line, not a row of cards */}
          <dl className="flex flex-wrap items-baseline gap-x-10 gap-y-2 border-b border-rule pb-4">
            <div className="flex items-baseline gap-2">
              <dt className="text-sm text-ink-soft">Open slots</dt>
              <dd className="font-mono text-xl font-semibold tabular-nums">
                {home.loading ? "—" : openSlots}
              </dd>
            </div>
            <div className="flex items-baseline gap-2">
              <dt className="text-sm text-ink-soft">Upcoming confirmed</dt>
              <dd className="font-mono text-xl font-semibold tabular-nums">
                {home.loading ? "—" : (data?.upcomingTotal ?? 0)}
              </dd>
            </div>
            <div className="flex items-baseline gap-2">
              <dt className="text-sm text-ink-soft">All bookings</dt>
              <dd className="font-mono text-xl font-semibold tabular-nums">
                {home.loading ? "—" : (data?.totalBookings ?? 0)}
              </dd>
            </div>
          </dl>

          {/* Upcoming bookings */}
          <section aria-labelledby="provider-upcoming-heading">
            <div className="flex items-center justify-between">
              <h2 id="provider-upcoming-heading" className="text-[11px] font-semibold uppercase tracking-[0.14em] text-coral-strong">
                Next confirmed appointments
              </h2>
              <Button asChild variant="ghost" size="sm">
                <Link href="/provider/bookings">All bookings</Link>
              </Button>
            </div>

            {home.loading ? (
              <div className="mt-3 space-y-3">
                {Array.from({ length: 3 }).map((_, index) => (
                  <Skeleton key={index} className="h-16 w-full rounded-lg" />
                ))}
              </div>
            ) : data && data.upcoming.length > 0 ? (
              <ul className="mt-3 divide-y rounded-lg border border-rule bg-card">
                {data.upcoming.map((booking) => (
                  <li
                    key={booking.id}
                    className="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{booking.customer_email}</p>
                      <p className="mt-0.5 font-mono text-xs tabular-nums text-ink-soft">
                        {formatSlotRange(booking.slot_starts_at, booking.slot_ends_at)}
                      </p>
                    </div>
                    <StatusBadge status={booking.status} />
                  </li>
                ))}
              </ul>
            ) : (
              <EmptyState
                className="mt-3"
                icon={ClipboardListIcon}
                title="No confirmed appointments yet"
                description="Publish open slots and confirmed bookings will land here the moment customers take them."
                action={
                  <Button asChild variant="outline" size="sm">
                    <Link href="/provider/availability">Open your calendar</Link>
                  </Button>
                }
              />
            )}
          </section>
        </>
      )}
    </div>
  );
}
