"use client";

import { useState } from "react";
import { AlertCircleIcon, Loader2Icon } from "lucide-react";
import { toast } from "sonner";

import { createProviderSlot } from "@/lib/api/endpoints";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { localInputToIso, timeZoneAbbr } from "@/lib/format";
import { createSlotSchema } from "@/lib/validations";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export interface CreateSlotInitial {
  date: string; // yyyy-mm-dd in the provider's timezone
  startTime: string; // HH:mm
  endTime: string; // HH:mm
}

interface CreateSlotDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initial: CreateSlotInitial;
  timezone: string;
  onCreated: () => void;
}

/**
 * Publish a new availability slot. `initial` is applied on every open —
 * parents remount via `key` when the prefill changes.
 */
export function CreateSlotDialog({
  open,
  onOpenChange,
  initial,
  timezone,
  onCreated,
}: CreateSlotDialogProps) {
  const [date, setDate] = useState(initial.date);
  const [startTime, setStartTime] = useState(initial.startTime);
  const [endTime, setEndTime] = useState(initial.endTime);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);

    const parsed = createSlotSchema.safeParse({ date, startTime, endTime });
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "Invalid slot details");
      return;
    }

    const startsAt = localInputToIso(`${date}T${startTime}`, timezone);
    const endsAt = localInputToIso(`${date}T${endTime}`, timezone);

    if (new Date(startsAt).getTime() <= Date.now()) {
      setError("Start time must be in the future");
      return;
    }

    setPending(true);
    try {
      await createProviderSlot({ starts_at: startsAt, ends_at: endsAt });
      toast.success("Slot published", {
        description: "Customers can book it from public search immediately.",
      });
      onCreated();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError("This slot overlaps an existing open slot. Adjust the times.");
      } else if (err instanceof ApiError && err.status === 400) {
        setError(err.message);
      } else {
        setError(toErrorMessage(err));
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) {
          setError(null);
          onOpenChange(next);
        }
      }}
    >
      <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
        <DialogHeader>
          <DialogTitle>New availability slot</DialogTitle>
          <DialogDescription>
            Published slots appear in public search immediately and can be
            booked instantly. Times are in {timezone} ({timeZoneAbbr(timezone)}).
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="slot-date">Date</Label>
            <Input
              id="slot-date"
              type="date"
              value={date}
              min={new Date().toISOString().slice(0, 10)}
              onChange={(event) => setDate(event.target.value)}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="slot-start">Start</Label>
              <Input
                id="slot-start"
                type="time"
                value={startTime}
                onChange={(event) => setStartTime(event.target.value)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="slot-end">End</Label>
              <Input
                id="slot-end"
                type="time"
                value={endTime}
                onChange={(event) => setEndTime(event.target.value)}
              />
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
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={pending}>
            Cancel
          </Button>
          <Button variant="brand" onClick={() => void submit()} disabled={pending}>
            {pending ? <Loader2Icon className="animate-spin" aria-hidden="true" /> : null}
            Publish slot
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
