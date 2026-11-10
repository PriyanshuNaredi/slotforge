import { bookingStatusMeta } from "@/lib/booking-status";
import { cn } from "@/lib/utils";

/**
 * Status stamps: squared, uppercase, letter-spaced — a departure-board
 * stamp rather than a pill.
 */
export function StatusBadge({
  status,
  className,
}: {
  status: string;
  className?: string;
}) {
  const meta = bookingStatusMeta(status);
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-sm px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ring-1 ring-inset",
        meta.classes,
        className,
      )}
    >
      <span className={cn("size-1.5 rounded-[1px]", meta.dot)} aria-hidden="true" />
      {meta.label}
    </span>
  );
}
