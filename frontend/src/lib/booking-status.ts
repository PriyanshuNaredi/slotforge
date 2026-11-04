import type { BookingStatus } from "./api/types";

export interface BookingStatusMeta {
  label: string;
  /** Tailwind classes for a badge: background, text, ring. */
  classes: string;
  /** Dot color for table/list rows. */
  dot: string;
}

export const BOOKING_STATUS_META: Record<BookingStatus, BookingStatusMeta> = {
  pending: {
    label: "Pending",
    classes: "bg-amber-50 text-amber-700 ring-amber-600/20",
    dot: "bg-amber-500",
  },
  confirmed: {
    label: "Confirmed",
    classes: "bg-emerald-50 text-emerald-700 ring-emerald-600/20",
    dot: "bg-emerald-500",
  },
  cancelled: {
    label: "Cancelled",
    classes: "bg-neutral-100 text-neutral-600 ring-neutral-500/20",
    dot: "bg-neutral-400",
  },
  completed: {
    label: "Completed",
    classes: "bg-sky-50 text-sky-700 ring-sky-600/20",
    dot: "bg-sky-500",
  },
  no_show: {
    label: "No-show",
    classes: "bg-red-50 text-red-700 ring-red-600/20",
    dot: "bg-red-500",
  },
};

export function bookingStatusMeta(status: string): BookingStatusMeta {  return (
    BOOKING_STATUS_META[status as BookingStatus] ?? {
      label: status,
      classes: "bg-neutral-100 text-neutral-600 ring-neutral-500/20",
      dot: "bg-neutral-400",
    }
  );
}

/**
 * Allowed provider transitions, mirroring the backend rules
 * (internal/providers/service.go): never back to pending; only
 * confirmed bookings can move to completed/no_show/cancelled.
 */
export function providerCanTransition(status: string): boolean {
  return status === "confirmed" || status === "pending";
}

/** Only confirmed bookings can be cancelled by the customer (backend rule). */
export function customerCanCancel(status: string): boolean {
  return status === "confirmed";
}
