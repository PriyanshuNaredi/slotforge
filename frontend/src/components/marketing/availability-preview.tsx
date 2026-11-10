"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ArrowRightIcon,
  CalendarPlusIcon,
  TriangleAlertIcon,
} from "lucide-react";

import { searchAvailableSlots } from "@/lib/api/endpoints";
import type { AvailableSlot } from "@/lib/api/types";
import { toErrorMessage } from "@/lib/api/client";
import { formatDuration, formatSlotRange } from "@/lib/format";
import { Skeleton } from "@/components/ui/skeleton";
import { useSession } from "@/components/providers/session-provider";

type State =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { kind: "ready"; slots: AvailableSlot[] };

/**
 * Live public availability, straight from GET /api/v1/slots — one card per
 * open slot, with a staggered entrance and a hover lift.
 */
export function AvailabilityPreview({ limit = 6 }: { limit?: number }) {
  const [state, setState] = useState<State>({ kind: "loading" });
  const { user } = useSession();

  useEffect(() => {
    let active = true;
    searchAvailableSlots({ limit })
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
  }, [limit]);

  const bookHref = (slotId: string) =>
    user ? `/dashboard/discover?slot=${slotId}` : `/register?slot=${slotId}`;

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
      <div className="flex items-center gap-3 rounded-xl border bg-card/60 px-5 py-8">
        <TriangleAlertIcon className="size-5 shrink-0 text-pending" aria-hidden="true" />
        <p className="text-sm text-ink-soft">
          Live availability is unreachable right now ({state.message.toLowerCase()}).
        </p>
      </div>
    );
  }

  if (state.kind === "empty") {
    return (
      <div className="rounded-xl border bg-card/60 px-6 py-10 text-center">
        <p className="text-sm font-semibold">No open slots right now</p>
        <p className="mx-auto mt-1 max-w-sm text-sm text-ink-soft">
          Be the first provider on SlotForge — publish your availability and
          it appears here instantly.
        </p>
        <Link
          href="/register"
          className="mt-4 inline-flex items-center gap-1.5 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-coral"
        >
          Start taking bookings
          <ArrowRightIcon aria-hidden="true" />
        </Link>
      </div>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {state.slots.map((slot, index) => (
        <article
          key={slot.id}
          style={{ animationDelay: `${index * 70}ms` }}
          className="animate-fade-up group flex flex-col justify-between rounded-xl border bg-card p-5 transition-[box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:shadow-lg hover:shadow-navy/10"
        >
          <div>
            <div className="flex items-start justify-between gap-2">
              <h3 className="truncate text-sm font-semibold">{slot.provider_name}</h3>
              <span className="shrink-0 rounded-full bg-brand-soft px-2 py-0.5 text-[11px] font-medium text-brand-strong">
                {formatDuration(slot.starts_at, slot.ends_at)}
              </span>
            </div>
            <p className="mt-2 flex items-start gap-1.5 text-sm text-ink-soft">
              <CalendarPlusIcon
                className="mt-0.5 size-4 shrink-0 text-brand-strong"
                aria-hidden="true"
              />
              {formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)}
            </p>
          </div>
          <Link
            href={bookHref(slot.id)}
            className="mt-4 inline-flex w-full items-center justify-center gap-1.5 rounded-md border border-input bg-card px-3 py-2 text-sm font-medium text-ink transition-colors hover:border-coral hover:text-brand-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-coral"
          >
            Book this slot
            <ArrowRightIcon
              className="size-4 transition-transform group-hover:translate-x-0.5"
              aria-hidden="true"
            />
          </Link>
        </article>
      ))}
    </div>
  );
}
