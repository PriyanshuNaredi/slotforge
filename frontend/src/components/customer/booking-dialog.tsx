"use client";

import Link from "next/link";
import { useState } from "react";
import {
  AlertCircleIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  Loader2Icon,
  UserIcon,
} from "lucide-react";

import { ApiError, toErrorMessage } from "@/lib/api/client";
import { createBooking } from "@/lib/api/endpoints";
import type { AvailableSlot } from "@/lib/api/types";
import { bookingNotesSchema } from "@/lib/validations";
import { formatDuration, formatFullDate, formatTime, timeZoneAbbr } from "@/lib/format";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

interface BookingDialogProps {
  slot: AvailableSlot | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onBooked?: (slotId: string) => void;
}

export function BookingDialog({ slot, open, onOpenChange, onBooked }: BookingDialogProps) {
  const [notes, setNotes] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [bookedSlotId, setBookedSlotId] = useState<string | null>(null);

  function reset() {
    setNotes("");
    setError(null);
    setBookedSlotId(null);
  }

  function handleOpenChange(next: boolean) {
    if (!next) reset();
    onOpenChange(next);
  }

  async function confirm() {
    if (!slot) return;

    const parsed = bookingNotesSchema.safeParse({ notes });
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "Invalid notes");
      return;
    }

    setPending(true);
    setError(null);
    try {
      await createBooking({
        slot_id: slot.id,
        notes: parsed.data.notes ? parsed.data.notes : undefined,
      });
      setBookedSlotId(slot.id);
      onBooked?.(slot.id);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError("This slot was just booked by someone else. Pick another time.");
      } else if (err instanceof ApiError && err.status === 404) {
        setError("This slot is no longer available. It may have been closed.");
      } else {
        setError(toErrorMessage(err));
      }
    } finally {
      setPending(false);
    }
  }

  const booked = bookedSlotId !== null;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
        {booked ? (
          <div className="py-2 text-center sm:text-left">
            <div className="mx-auto flex size-11 items-center justify-center rounded-full bg-emerald-500/15 sm:mx-0">
              <CheckCircle2Icon className="size-6 text-emerald-600" aria-hidden="true" />
            </div>
            <DialogHeader className="mt-4 sm:text-left">
              <DialogTitle>Booking confirmed</DialogTitle>
              <DialogDescription>
                Your appointment is locked in. The provider sees it on their
                dashboard immediately.
              </DialogDescription>
            </DialogHeader>
            <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button variant="outline" onClick={() => handleOpenChange(false)}>
                Done
              </Button>
              <Button asChild variant="brand">
                <Link href={`/dashboard/bookings/${bookedSlotId}`}>
                  View booking
                </Link>
              </Button>
            </div>
          </div>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Confirm your booking</DialogTitle>
              <DialogDescription>
                Confirmations are instant — the slot locks to you the moment
                you book.
              </DialogDescription>
            </DialogHeader>

            {slot ? (
              <div className="rounded-lg border bg-background p-4">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <UserIcon className="size-4 text-muted-foreground" aria-hidden="true" />
                  {slot.provider_name}
                </div>
                <div className="mt-2 flex items-start gap-2 text-sm text-muted-foreground">
                  <CalendarClockIcon
                    className="mt-0.5 size-4 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <span>
                    {formatFullDate(new Date(slot.starts_at), slot.timezone)}
                    <br />
                    {formatTime(new Date(slot.starts_at), slot.timezone)} –{" "}
                    {formatTime(new Date(slot.ends_at), slot.timezone)} (
                    {timeZoneAbbr(slot.timezone, new Date(slot.starts_at))}) ·{" "}
                    {formatDuration(slot.starts_at, slot.ends_at)}
                  </span>
                </div>
              </div>
            ) : null}

            <div className="space-y-1.5">
              <Label htmlFor="booking-notes">
                Notes for the provider <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Textarea
                id="booking-notes"
                placeholder="Anything the provider should know ahead of time"
                rows={3}
                value={notes}
                onChange={(event) => setNotes(event.target.value)}
                aria-invalid={Boolean(error)}
              />
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

            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={pending}>
                Cancel
              </Button>
              <Button variant="brand" onClick={() => void confirm()} disabled={pending || !slot}>
                {pending ? (
                  <Loader2Icon className="animate-spin" aria-hidden="true" />
                ) : null}
                Confirm booking
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
