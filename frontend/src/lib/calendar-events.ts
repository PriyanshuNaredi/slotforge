import type { EventInput } from "@fullcalendar/react";

import type { AvailableSlot, Booking, ProviderBooking, Slot } from "./api/types";
import { bookingStatusMeta } from "./booking-status";

/**
 * Map backend rows to FullCalendar events. FullCalendar v7's classic theme
 * renders timeGrid events as solid `color` blocks with `contrastColor` text,
 * and month-view dots in solid `color`. Every pair here is checked for
 * 4.5:1 text contrast — the old table set `contrast` equal to `color` and
 * rendered unreadable events.
 */

interface EventColors {
  /** Solid accent: event background and month-dot color. */
  color: string;
  /** Text on the solid background. */
  contrast: string;
}

/** SlotForge palette per booking/slot state (from the design tokens). */
export const EVENT_COLORS: Record<string, EventColors> = {
  "slot:open": { color: "#bc3e1b", contrast: "#fff8f2" },
  "slot:closed": { color: "#9fb2bd", contrast: "#1f3049" },
  confirmed: { color: "#2f5d46", contrast: "#f2eee3" },
  completed: { color: "#41597a", contrast: "#f2eee3" },
  pending: { color: "#9a6110", contrast: "#fff8f2" },
  cancelled: { color: "#b6c2cb", contrast: "#1f3049" },
  no_show: { color: "#b3362b", contrast: "#fff8f2" },
};

/** Legend rows for the calendar toolbar, in display order. */
export const CALENDAR_LEGEND: { label: string; color: string }[] = [
  { label: "Open slot", color: EVENT_COLORS["slot:open"].color },
  { label: "Confirmed", color: EVENT_COLORS.confirmed.color },
  { label: "Pending", color: EVENT_COLORS.pending.color },
  { label: "Completed", color: EVENT_COLORS.completed.color },
  { label: "Cancelled", color: EVENT_COLORS.cancelled.color },
  { label: "No-show", color: EVENT_COLORS.no_show.color },
];

function withColors(event: EventInput, colors: EventColors): EventInput {
  return {
    ...event,
    color: colors.color,
    contrastColor: colors.contrast,
  };
}

export function providerSlotEvents(slots: Slot[]): EventInput[] {
  return slots.map((slot) => {
    const open = slot.status === "open";
    return withColors(
      {
        id: `slot:${slot.id}`,
        title: open ? "Open slot" : "Closed slot",
        start: slot.starts_at,
        end: slot.ends_at,
        extendedProps: { kind: "slot" as const, refId: slot.id, status: slot.status },
      },
      open ? EVENT_COLORS["slot:open"] : EVENT_COLORS["slot:closed"],
    );
  });
}

const PROVIDER_STATUS_KEYS: Record<string, string> = {
  confirmed: "confirmed",
  completed: "completed",
  pending: "pending",
  cancelled: "cancelled",
  no_show: "no_show",
};

/** Full emails truncate to "custome…" in event chips — show the local part. */
function shortName(email: string): string {
  return email.split("@")[0] || email;
}

export function providerBookingEvents(bookings: ProviderBooking[]): EventInput[] {
  return bookings
    .filter((booking) => PROVIDER_STATUS_KEYS[booking.status])
    .map((booking) =>
      withColors(
        {
          id: `booking:${booking.id}`,
          title: shortName(booking.customer_email),
          start: booking.slot_starts_at,
          end: booking.slot_ends_at,
          extendedProps: { kind: "booking" as const, refId: booking.id, status: booking.status },
        },
        EVENT_COLORS[booking.status],
      ),
    );
}

export function customerBookingEvents(
  bookings: Booking[],
  slots: Record<string, AvailableSlot>,
): EventInput[] {
  return bookings
    .filter((booking) => PROVIDER_STATUS_KEYS[booking.status] && slots[booking.slot_id])
    .map((booking) => {
      const slot = slots[booking.slot_id];
      return withColors(
        {
          id: `booking:${booking.id}`,
          title: slot.provider_name,
          start: slot.starts_at,
          end: slot.ends_at,
          extendedProps: { kind: "booking" as const, refId: booking.id, status: booking.status },
        },
        EVENT_COLORS[booking.status],
      );
    });
}

export function eventStatusLabel(status: string | undefined): string {
  return bookingStatusMeta(status ?? "").label;
}
