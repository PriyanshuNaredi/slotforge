"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { CalendarDaysIcon, CalendarSearchIcon } from "lucide-react";

import { listCustomerBookings } from "@/lib/api/endpoints";
import { customerBookingEvents } from "@/lib/calendar-events";
import { timeZoneAbbr } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { SlotCalendar } from "@/components/shared/slot-calendar";
import { Button } from "@/components/ui/button";
import { useAsyncData } from "@/hooks/use-async-data";
import { useSlotDetails } from "@/hooks/use-slot-details";

function viewerTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return "UTC";
  }
}

/** Customer calendar: upcoming appointments and booking history. */
export default function CustomerCalendarPage() {
  const router = useRouter();

  const list = useAsyncData("customer-calendar", () =>
    listCustomerBookings({ limit: 100 }),
  );

  const bookings = list.data?.bookings ?? [];
  const { slots } = useSlotDetails(bookings.map((booking) => booking.slot_id));

  // Hydration yields null for expired slots; only placeable bookings become events.
  const placeable: Record<string, NonNullable<(typeof slots)[string]>> = {};
  for (const [id, slot] of Object.entries(slots)) {
    if (slot) placeable[id] = slot;
  }

  const events = customerBookingEvents(bookings, placeable);
  // Past/closed slots 404 on the public endpoint, so their bookings can't be
  // placed on the calendar — surface that honestly instead of hiding rows.
  const unplaced = bookings.length - events.length;
  const timezone = viewerTimezone();

  return (
    <div className="mx-auto max-w-6xl space-y-4">
      <PageHeader
        title="Calendar"
        description={`Your appointments in your timezone: ${timezone} (${timeZoneAbbr(timezone)})`}
        actions={
          <Button asChild size="sm">
            <Link href="/dashboard/discover">
              <CalendarSearchIcon aria-hidden="true" />
              Find a slot
            </Link>
          </Button>
        }
      />

      {unplaced > 0 ? (
        <p className="rounded-lg border border-dashed bg-card/60 px-4 py-2.5 text-sm text-muted-foreground">
          {unplaced} older appointment{unplaced === 1 ? " is" : "s are"} not shown —
          their slot details are no longer published. See{" "}
          <Link href="/dashboard/bookings" className="font-medium underline underline-offset-4">
            My bookings
          </Link>{" "}
          for the full history.
        </p>
      ) : null}

      <div className="rounded-xl border bg-card p-4 shadow-xs">
        <SlotCalendar
          events={events}
          loading={list.loading}
          error={list.error}
          onRetry={list.reload}
          timeZone={timezone}
          desktopInitialView="dayGridMonth"
          onEventClick={(eventId) => {
            if (eventId.startsWith("booking:")) {
              router.push(`/dashboard/bookings/${eventId.slice("booking:".length)}`);
            }
          }}
          emptyIcon={CalendarDaysIcon}
          emptyTitle="Nothing on your calendar"
          emptyDescription="Book an open slot and it lands here with its full history."
          ariaLabel="Customer calendar"
        />
      </div>
    </div>
  );
}
