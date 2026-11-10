"use client";

import { useState } from "react";
import {
  CalendarRangeIcon,
  CheckCircle2Icon,
  CircleXIcon,
  UserXIcon,
} from "lucide-react";
import { toast } from "sonner";

import { updateProviderBookingStatus } from "@/lib/api/endpoints";
import type { ProviderBooking } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { bookingStatusMeta, providerCanTransition } from "@/lib/booking-status";
import { formatDuration, formatSlotRange } from "@/lib/format";
import { StatusBadge } from "@/components/shared/status-badge";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";

type ProviderStatus = "confirmed" | "cancelled" | "completed" | "no_show";

interface BookingDetailDialogProps {
  booking: ProviderBooking | null;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful status change so owners can refresh data. */
  onUpdated?: () => void;
}

/** Provider booking details + valid status transitions (backend matrix). */
export function BookingDetailDialog({
  booking,
  onOpenChange,
  onUpdated,
}: BookingDetailDialogProps) {
  const [transitionTarget, setTransitionTarget] = useState<ProviderStatus | null>(null);
  const [transitioning, setTransitioning] = useState(false);

  async function confirmTransition() {
    if (!booking || !transitionTarget) return;
    setTransitioning(true);
    try {
      await updateProviderBookingStatus(booking.id, transitionTarget);
      toast.success(
        `Booking marked as ${bookingStatusMeta(transitionTarget).label.toLowerCase()}`,
      );
      setTransitionTarget(null);
      onOpenChange(false);
      onUpdated?.();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("That status change isn't allowed for this booking anymore.");
      } else {
        toast.error(toErrorMessage(err));
      }
      setTransitionTarget(null);
    } finally {
      setTransitioning(false);
    }
  }

  return (
    <>
      <Dialog open={booking !== null} onOpenChange={(open) => !open && onOpenChange(false)}>
        <DialogContent className="sm:max-w-lg">
          {booking ? (
            <>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2.5">
                  {booking.customer_email}
                  <StatusBadge status={booking.status} />
                </DialogTitle>
                <DialogDescription>
                  Booked{" "}
                  {new Date(booking.created_at).toLocaleString("en-US", {
                    month: "short",
                    day: "numeric",
                    hour: "numeric",
                    minute: "2-digit",
                  })}
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4">
                <div className="rounded-md border border-rule bg-background p-4">
                  <p className="flex items-center gap-2 font-mono text-sm font-medium tabular-nums">
                    <CalendarRangeIcon className="size-4 text-coral-strong" aria-hidden="true" />
                    {formatSlotRange(booking.slot_starts_at, booking.slot_ends_at)}
                  </p>
                  <p className="mt-1 text-xs text-ink-soft">
                    {formatDuration(booking.slot_starts_at, booking.slot_ends_at)} · Slot #
                    {booking.slot_id.slice(0, 8)}
                  </p>
                </div>

                <div>
                  <p className="text-sm font-medium">Customer notes</p>
                  <p className="mt-1 text-sm text-ink-soft">
                    {booking.notes?.trim() ? booking.notes : "No notes added."}
                  </p>
                </div>

                <Separator />

                <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-sm">
                  <dt className="text-ink-soft">Booking ref</dt>
                  <dd className="font-mono text-xs leading-6">{booking.id.slice(0, 8)}</dd>
                  <dt className="text-ink-soft">Last update</dt>
                  <dd>
                    {new Date(booking.updated_at).toLocaleString("en-US", {
                      month: "short",
                      day: "numeric",
                      hour: "numeric",
                      minute: "2-digit",
                    })}
                  </dd>
                </dl>
              </div>

              {providerCanTransition(booking.status) ? (
                <div className="flex flex-wrap justify-end gap-2">
                  {booking.status === "pending" ? (
                    <Button
                      size="sm"
                      variant="brand"
                      onClick={() => setTransitionTarget("confirmed")}
                    >
                      <CheckCircle2Icon aria-hidden="true" />
                      Confirm
                    </Button>
                  ) : null}
                  {booking.status === "confirmed" ? (
                    <>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setTransitionTarget("completed")}
                      >
                        <CheckCircle2Icon aria-hidden="true" />
                        Mark completed
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setTransitionTarget("no_show")}
                      >
                        <UserXIcon aria-hidden="true" />
                        Mark no-show
                      </Button>
                    </>
                  ) : null}
                  <Button
                    size="sm"
                    variant="outline"
                    className="text-destructive hover:bg-destructive/5 hover:text-destructive"
                    onClick={() => setTransitionTarget("cancelled")}
                  >
                    <CircleXIcon aria-hidden="true" />
                    Cancel booking
                  </Button>
                </div>
              ) : null}
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={transitionTarget !== null}
        onOpenChange={(open) => !open && setTransitionTarget(null)}
        icon={transitionTarget === "cancelled" ? CircleXIcon : CheckCircle2Icon}
        title={
          transitionTarget
            ? `Mark as ${bookingStatusMeta(transitionTarget).label.toLowerCase()}?`
            : ""
        }
        description={
          transitionTarget === "cancelled"
            ? "The customer loses this appointment and the slot reopens for others."
            : "The customer's booking reflects the new status immediately."
        }
        confirmLabel={transitionTarget ? bookingStatusMeta(transitionTarget).label : "Confirm"}
        destructive={transitionTarget === "cancelled"}
        pending={transitioning}
        onConfirm={() => void confirmTransition()}
      />
    </>
  );
}
