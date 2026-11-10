"use client";

import { CalendarClockIcon } from "lucide-react";

import type { AvailableSlot } from "@/lib/api/types";
import { formatDuration, formatSlotRange } from "@/lib/format";
import { Button } from "@/components/ui/button";

export function SlotCard({
  slot,
  onBook,
}: {
  slot: AvailableSlot;
  onBook: (slot: AvailableSlot) => void;
}) {
  return (
    <article className="group flex flex-col justify-between rounded-lg border border-rule bg-card p-4 transition-colors hover:border-rule-strong">
      <div>
        <div className="flex items-start justify-between gap-2">
          <h3 className="truncate text-sm font-semibold">{slot.provider_name}</h3>
          <span className="shrink-0 rounded-sm bg-brand-soft px-1.5 py-0.5 font-mono text-[11px] font-medium tabular-nums text-brand-strong">
            {formatDuration(slot.starts_at, slot.ends_at)}
          </span>
        </div>
        <p className="mt-2.5 flex items-start gap-1.5 font-mono text-sm tabular-nums text-ink-soft">
          <CalendarClockIcon
            className="mt-0.5 size-4 shrink-0 text-coral-strong"
            aria-hidden="true"
          />
          {formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)}
        </p>
      </div>
      <Button size="sm" className="mt-4 w-full" onClick={() => onBook(slot)}>
        Book this slot
      </Button>
    </article>
  );
}
