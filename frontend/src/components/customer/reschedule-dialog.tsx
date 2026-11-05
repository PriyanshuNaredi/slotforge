"use client";

import { useState } from "react";
import {
  AlertCircleIcon,
  ArrowRightIcon,
  CalendarClockIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  Loader2Icon,
} from "lucide-react";
import { toast } from "sonner";

import { rescheduleBooking, searchAvailableSlots } from "@/lib/api/endpoints";
import type { SlotSearchResult } from "@/lib/api/types";
import type { AvailableSlot, Booking } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { formatFullDate, formatDuration, formatSlotRange, formatTime } from "@/lib/format";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/use-async-data";

const PAGE_SIZE = 8;

interface RescheduleDialogProps {
  open: boolean;
  booking: Booking | null;
  /** Hydrated slot of the current booking — provides provider_id + times. */
  currentSlot: AvailableSlot | null;
  onRescheduled: () => void;
  onOpenChange: (open: boolean) => void;
}

/**
 * Reschedule flow: pick a new slot from the same provider, compare times,
 * confirm once via PATCH /bookings/:id/reschedule. 409s (slot just taken,
 * closed, cross-provider) return the customer to slot selection.
 */
export function RescheduleDialog({
  open: openProp,
  booking,
  currentSlot,
  onRescheduled,
  onOpenChange,
}: RescheduleDialogProps) {
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<AvailableSlot | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const open = openProp && booking !== null && currentSlot !== null;
  const list = useAsyncData<SlotSearchResult>(
    open ? `${booking!.id}#${page}` : null,
    () =>
      searchAvailableSlots({
        provider_id: currentSlot!.provider_id,
        from: new Date().toISOString(),
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
  );

  const slots: AvailableSlot[] = (list.data?.slots ?? []).filter(
    (slot) => slot.id !== currentSlot?.id,
  );
  const total = list.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function close() {
    setSelected(null);
    setError(null);
    setPage(0);
    onOpenChange(false);
  }

  async function confirm() {
    if (!booking || !selected) return;
    setConfirming(true);
    setError(null);
    try {
      const updated = await rescheduleBooking(booking.id, selected.id);
      toast.success("Appointment rescheduled", {
        description: `Moved to ${formatSlotRange(selected.starts_at, selected.ends_at, selected.timezone)}`,
      });
      close();
      onRescheduled();
      void updated;
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError(
          "That slot was just taken or is no longer available. Please select another time.",
        );
        setSelected(null);
        list.reload();
      } else {
        setError(toErrorMessage(err));
      }
    } finally {
      setConfirming(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Reschedule appointment</DialogTitle>
          <DialogDescription>
            Choose a new time with {currentSlot?.provider_name ?? "the provider"}.
            Your current appointment stays booked until you confirm the change.
          </DialogDescription>
        </DialogHeader>

        {selected ? (
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-[1fr_auto_1fr] sm:items-center">
              <div className="rounded-lg border border-dashed bg-muted/40 p-3">
                <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  Current
                </p>
                <p className="mt-1 text-sm font-medium">
                  {currentSlot
                    ? formatFullDate(new Date(currentSlot.starts_at), currentSlot.timezone)
                    : "—"}
                </p>
                <p className="text-sm text-muted-foreground">
                  {currentSlot
                    ? `${formatTime(new Date(currentSlot.starts_at), currentSlot.timezone)} – ${formatTime(new Date(currentSlot.ends_at), currentSlot.timezone)} · ${formatDuration(currentSlot.starts_at, currentSlot.ends_at)}`
                    : ""}
                </p>
              </div>
              <ArrowRightIcon
                className="mx-auto size-4 text-muted-foreground"
                aria-hidden="true"
              />
              <div className="rounded-lg border border-brand/40 bg-brand-soft/60 p-3">
                <p className="text-xs font-medium uppercase tracking-wide text-brand-strong">
                  New time
                </p>
                <p className="mt-1 text-sm font-medium">
                  {formatFullDate(new Date(selected.starts_at), selected.timezone)}
                </p>
                <p className="text-sm text-muted-foreground">
                  {formatTime(new Date(selected.starts_at), selected.timezone)} –{" "}
                  {formatTime(new Date(selected.ends_at), selected.timezone)} ·{" "}
                  {formatDuration(selected.starts_at, selected.ends_at)}
                </p>
              </div>
            </div>

            {error ? (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive"
              >
                <AlertCircleIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                {error}
              </div>
            ) : null}

            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setSelected(null)} disabled={confirming}>
                <ChevronLeftIcon aria-hidden="true" />
                Back to times
              </Button>
              <Button variant="brand" onClick={() => void confirm()} disabled={confirming}>
                {confirming ? (
                  <Loader2Icon className="animate-spin" aria-hidden="true" />
                ) : null}
                Confirm reschedule
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {error ? (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive"
              >
                <AlertCircleIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                {error}
              </div>
            ) : null}

            {list.loading && slots.length === 0 ? (
              <div className="space-y-2">
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton key={index} className="h-14 w-full rounded-lg" />
                ))}
              </div>
            ) : !list.loading && slots.length === 0 ? (
              <div className="rounded-lg border border-dashed bg-muted/40 px-4 py-6 text-center text-sm text-muted-foreground">
                No future availability from this provider right now. Check back
                soon — you can also keep your current appointment.
              </div>
            ) : (
              <>
                <ul
                  role="listbox"
                  aria-label="Available replacement slots"
                  className={`max-h-64 space-y-2 overflow-y-auto pr-1 ${
                    list.loading ? "opacity-60 transition-opacity" : ""
                  }`}
                >
                  {slots.map((slot) => (
                    <li key={slot.id}>
                      <button
                        type="button"
                        onClick={() => setSelected(slot)}
                        className="flex w-full items-center justify-between gap-3 rounded-lg border bg-background px-3.5 py-2.5 text-left transition-colors hover:border-brand hover:bg-brand-soft/40 focus-visible:ring-[3px] focus-visible:ring-ring/40 cursor-pointer"
                      >
                        <span className="flex items-center gap-2.5">
                          <CalendarClockIcon
                            className="size-4 shrink-0 text-muted-foreground"
                            aria-hidden="true"
                          />
                          <span className="text-sm">{formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)}</span>
                        </span>
                        <span className="shrink-0 text-xs text-muted-foreground">
                          {formatDuration(slot.starts_at, slot.ends_at)}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
                {pageCount > 1 ? (
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-muted-foreground" aria-live="polite">
                      Page {page + 1} of {pageCount} · {total} slots
                    </p>
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={page === 0}
                        onClick={() => setPage(page - 1)}
                      >
                        <ChevronLeftIcon aria-hidden="true" />
                        Prev
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={page >= pageCount - 1}
                        onClick={() => setPage(page + 1)}
                      >
                        Next
                        <ChevronRightIcon aria-hidden="true" />
                      </Button>
                    </div>
                  </div>
                ) : null}
              </>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
