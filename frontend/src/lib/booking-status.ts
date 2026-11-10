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
    classes: "bg-pending-wash text-pending ring-pending/25",
    dot: "bg-pending",
  },
  confirmed: {
    label: "Confirmed",
    classes: "bg-success-soft text-success ring-success/25",
    dot: "bg-success",
  },
  cancelled: {
    label: "Cancelled",
    classes: "bg-muted text-ink-soft ring-rule-strong",
    dot: "bg-rule-strong",
  },
  completed: {
    label: "Completed",
    classes: "bg-completed-wash text-completed ring-completed/25",
    dot: "bg-completed",
  },
  no_show: {
    label: "No-show",
    classes: "bg-destructive/10 text-destructive ring-destructive/25",
    dot: "bg-destructive",
  },
};

export function bookingStatusMeta(status: string): BookingStatusMeta {  return (
    BOOKING_STATUS_META[status as BookingStatus] ?? {
      label: status,
      classes: "bg-muted text-ink-soft ring-rule-strong",
      dot: "bg-rule-strong",
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
