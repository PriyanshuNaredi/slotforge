"use client";

import { CalendarClockIcon } from "lucide-react";

import type { AvailableSlot } from "@/lib/api/types";
import { formatDuration, formatSlotRange, timeZoneAbbr } from "@/lib/format";
import { Button } from "@/components/ui/button";

export function SlotCard({
  slot,
  onBook,
}: {
  slot: AvailableSlot;
  onBook: (slot: AvailableSlot) => void;
}) {
  return (
    <article className="group flex flex-col justify-between rounded-xl border bg-card p-5 transition-shadow hover:shadow-md">
      <div>
        <div className="flex items-start justify-between gap-2">
          <h3 className="truncate text-sm font-semibold">{slot.provider_name}</h3>
          <span className="shrink-0 rounded-full bg-brand-soft px-2 py-0.5 text-[11px] font-medium text-brand-strong">
            {formatDuration(slot.starts_at, slot.ends_at)}
          </span>
        </div>
        <p className="mt-2 flex items-start gap-1.5 text-sm text-muted-foreground">
          <CalendarClockIcon
            className="mt-0.5 size-4 shrink-0 text-muted-foreground/70"
            aria-hidden="true"
          />
          {formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)}
        </p>
        <p className="mt-1 pl-5.5 text-xs text-muted-foreground/80">
          {timeZoneAbbr(slot.timezone)} · {slot.timezone}
        </p>
      </div>
      <Button size="sm" className="mt-4 w-full" onClick={() => onBook(slot)}>
        Book this slot
      </Button>
    </article>
  );
}
