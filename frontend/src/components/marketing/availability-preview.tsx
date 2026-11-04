"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CalendarPlusIcon,
} from "lucide-react";

import { searchAvailableSlots } from "@/lib/api/endpoints";
import type { AvailableSlot } from "@/lib/api/types";
import { toErrorMessage } from "@/lib/api/client";
import { formatDuration, formatSlotRange, timeZoneAbbr } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/providers/session-provider";

type State =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { kind: "ready"; slots: AvailableSlot[] };

/** Live public availability, straight from GET /api/v1/slots. */
export function AvailabilityPreview() {
  const [state, setState] = useState<State>({ kind: "loading" });
  const { user } = useSession();

  useEffect(() => {
    let active = true;
    searchAvailableSlots({ limit: 6 })
      .then((result) => {
        if (!active) return;
        setState(
          result.slots.length > 0
            ? { kind: "ready", slots: result.slots }
            : { kind: "empty" },
        );
      })
      .catch((error) => {
        if (active) setState({ kind: "error", message: toErrorMessage(error) });
      });
    return () => {
      active = false;
    };
  }, []);

  if (state.kind === "loading") {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <div key={index} className="rounded-xl border bg-card p-5">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="mt-3 h-3 w-40" />
            <Skeleton className="mt-5 h-8 w-full" />
          </div>
        ))}
      </div>
    );
  }

  if (state.kind === "error") {
    return (
      <div className="flex items-center gap-3 rounded-xl border border-dashed bg-card/60 px-5 py-8">
        <AlertTriangleIcon className="size-5 shrink-0 text-amber-600" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">
          Live availability is unreachable right now ({state.message.toLowerCase()}).
        </p>
      </div>
    );
  }

  if (state.kind === "empty") {
    return (
      <div className="rounded-xl border border-dashed bg-card/60 px-6 py-10 text-center">
        <p className="text-sm font-medium">No open slots right now</p>
        <p className="mx-auto mt-1 max-w-sm text-sm text-muted-foreground">
          Be the first provider on SlotForge — publish your availability and
          it appears here instantly.
        </p>
        <Button asChild variant="outline" size="sm" className="mt-4">
          <Link href="/register">Start taking bookings</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {state.slots.map((slot) => (
        <article
          key={slot.id}
          className="group flex flex-col justify-between rounded-xl border bg-card p-5 transition-shadow hover:shadow-md"
        >
          <div>
            <div className="flex items-center justify-between gap-2">
              <h3 className="truncate text-sm font-semibold">{slot.provider_name}</h3>
              <span className="shrink-0 rounded-full bg-brand-soft px-2 py-0.5 text-[11px] font-medium text-brand-strong">
                {formatDuration(slot.starts_at, slot.ends_at)}
              </span>
            </div>
            <p className="mt-2 text-sm text-muted-foreground">
              {formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)}
            </p>
            <p className="mt-1 text-xs text-muted-foreground/80">
              Times shown in {timeZoneAbbr(slot.timezone)} · {slot.timezone}
            </p>
          </div>
          <Button asChild variant="outline" size="sm" className="mt-4 w-full">
            <Link
              href={
                user
                  ? `/dashboard/discover?slot=${slot.id}`
                  : `/register?slot=${slot.id}`
              }
            >
              <CalendarPlusIcon aria-hidden="true" />
              Book this slot
              <ArrowRightIcon
                aria-hidden="true"
                className="transition-transform group-hover:translate-x-0.5"
              />
            </Link>
          </Button>
        </article>
      ))}
    </div>
  );
}
