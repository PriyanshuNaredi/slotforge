import type { EventInput } from "@fullcalendar/react";

import type { AvailableSlot, Booking, ProviderBooking, Slot } from "./api/types";
import { bookingStatusMeta } from "./booking-status";

/**
 * Map backend rows to FullCalendar events. FullCalendar v7's classic theme
 * derives event styling from `color` (solid accents: dots, 15% tinted
 * backgrounds) and `contrastColor` (text) — no class hooks exist.
 */

interface EventColors {
  /** Solid accent: dot color and tinted background base. */
  color: string;
  /** Text on the tinted background. */
  contrast: string;
}

/** SlotForge palette per booking/slot state. */
export const EVENT_COLORS: Record<string, EventColors> = {
  "slot:open": { color: "#F17C69", contrast: "#172B4D" },
  "slot:closed": { color: "#9FB2BD", contrast: "#44597E" },
  confirmed: { color: "#344B43", contrast: "#344B43" },
  completed: { color: "#172B4D", contrast: "#172B4D" },
  pending: { color: "#D97706", contrast: "#7C5410" },
  cancelled: { color: "#B6C2CB", contrast: "#67798F" },
  no_show: { color: "#B3362B", contrast: "#8C3A30" },
};

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

export function providerBookingEvents(bookings: ProviderBooking[]): EventInput[] {
  return bookings
    .filter((booking) => PROVIDER_STATUS_KEYS[booking.status])
    .map((booking) =>
      withColors(
        {
          id: `booking:${booking.id}`,
          title: booking.customer_email,
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
