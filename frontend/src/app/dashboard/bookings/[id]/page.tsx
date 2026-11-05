"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import {
  ArrowLeftIcon,
  CalendarClockIcon,
  CalendarX2Icon,
  FileQuestionIcon,
  UserIcon,
} from "lucide-react";

import { cancelBooking, getAvailableSlot, getBooking } from "@/lib/api/endpoints";
import type { Booking } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { customerCanCancel } from "@/lib/booking-status";
import { formatDuration, formatFullDate, formatRelative, formatTime, timeZoneAbbr } from "@/lib/format";
import { StatusBadge } from "@/components/shared/status-badge";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { ErrorState } from "@/components/shared/states";
import { RescheduleDialog } from "@/components/customer/reschedule-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { useAsyncData } from "@/hooks/use-async-data";

export default function BookingDetailPage() {
  const params = useParams<{ id: string }>();

  const booking = useAsyncData(params.id ?? null, () => getBooking(params.id));
  const slot = useAsyncData(
    booking.data?.slot_id ?? null,
    () => getAvailableSlot(booking.data!.slot_id),
  );

  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [rescheduleOpen, setRescheduleOpen] = useState(false);

  async function confirmCancel() {
    if (!booking.data) return;
    setCancelling(true);
    try {
      await cancelBooking(booking.data.id);
      toast.success("Booking cancelled", {
        description: "The slot is open for others again.",
      });
      setCancelOpen(false);
      booking.reload();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("This booking can no longer be cancelled.");
      } else {
        toast.error(toErrorMessage(err));
      }
      setCancelOpen(false);
    } finally {
      setCancelling(false);
    }
  }

  if (booking.loading) {
    return (
      <div className="mx-auto max-w-2xl space-y-4">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-64 w-full rounded-xl" />
      </div>
    );
  }

  if (booking.error || !booking.data) {
    return (
      <div className="mx-auto max-w-2xl">
        <ErrorState
          icon={FileQuestionIcon}
          title="Booking not found"
          description={
            booking.error ??
            "This booking doesn't exist or belongs to another account."
          }
          action={
            <Button asChild variant="outline" size="sm">
              <Link href="/dashboard/bookings">Back to my bookings</Link>
            </Button>
          }
        />
      </div>
    );
  }

  const data: Booking = booking.data;
  const slotData = slot.data ?? null;

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Button asChild variant="ghost" size="sm" className="-ml-2">
        <Link href="/dashboard/bookings">
          <ArrowLeftIcon aria-hidden="true" />
          All bookings
        </Link>
      </Button>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {slotData?.provider_name ?? "Booking"}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Booked {formatRelative(new Date(data.created_at))} · #
            {data.id.slice(0, 8)}
          </p>
        </div>
        <StatusBadge status={data.status} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Appointment</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {slotData ? (
            <>
              <div className="flex items-start gap-3">
                <CalendarClockIcon
                  className="mt-0.5 size-4 text-muted-foreground"
                  aria-hidden="true"
                />
                <div>
                  <p className="text-sm font-medium">
                    {formatFullDate(new Date(slotData.starts_at), slotData.timezone)}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    {formatTime(new Date(slotData.starts_at), slotData.timezone)} –{" "}
                    {formatTime(new Date(slotData.ends_at), slotData.timezone)} ·{" "}
                    {formatDuration(slotData.starts_at, slotData.ends_at)}
                  </p>
                  <p className="mt-0.5 text-xs text-muted-foreground/80">
                    Provider timezone: {slotData.timezone} (
                    {timeZoneAbbr(slotData.timezone, new Date(slotData.starts_at))})
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <UserIcon className="mt-0.5 size-4 text-muted-foreground" aria-hidden="true" />
                <div>
                  <p className="text-sm font-medium">{slotData.provider_name}</p>
                  <p className="text-xs text-muted-foreground">Provider on SlotForge</p>
                </div>
              </div>
            </>
          ) : slot.loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-56" />
              <Skeleton className="h-4 w-40" />
            </div>
          ) : (
            <p className="rounded-lg border border-dashed bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
              The time details for this booking aren&apos;t available anymore —
              the slot has closed or is in the past.
            </p>
          )}

          <Separator />

          <div>
            <p className="text-sm font-medium">Your notes</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {data.notes?.trim() ? data.notes : "No notes added."}
            </p>
          </div>

          <Separator />

          <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-sm">
            <dt className="text-muted-foreground">Booking ID</dt>
            <dd className="font-mono text-xs leading-6">{data.id}</dd>
            <dt className="text-muted-foreground">Last updated</dt>
            <dd>{formatRelative(new Date(data.updated_at))}</dd>
          </dl>
        </CardContent>
      </Card>

      {customerCanCancel(data.status) ? (
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="outline"
            onClick={() => setRescheduleOpen(true)}
          >
            <CalendarClockIcon aria-hidden="true" />
            Reschedule
          </Button>
          <Button
            variant="outline"
            className="text-destructive hover:bg-destructive/5 hover:text-destructive"
            onClick={() => setCancelOpen(true)}
          >
            <CalendarX2Icon aria-hidden="true" />
            Cancel booking
          </Button>
        </div>
      ) : null}

      {slotData ? (
        <RescheduleDialog
          open={rescheduleOpen}
          onOpenChange={setRescheduleOpen}
          booking={data}
          currentSlot={slotData}
          onRescheduled={() => {
            booking.reload();
            slot.reload();
          }}
        />
      ) : null}

      <ConfirmDialog
        open={cancelOpen}
        onOpenChange={setCancelOpen}
        icon={CalendarX2Icon}
        title="Cancel this booking?"
        description="Your appointment will be cancelled and the slot reopened for other customers. This can't be undone."
        confirmLabel="Cancel booking"
        destructive
        pending={cancelling}
        onConfirm={() => void confirmCancel()}
      />
    </div>
  );
}
