"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  ArrowRightIcon,
  Loader2Icon,
  CalendarPlusIcon,
  CalendarX2Icon,
  LockIcon,
  UserRoundIcon,
} from "lucide-react";
import { toast } from "sonner";

import { closeProviderSlot, listProviderSlots, getMyProviderProfile } from "@/lib/api/endpoints";
import type { ProviderProfile, Slot } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { formatDuration, formatTime, isoToLocalInput, timeZoneAbbr } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { MonthCalendar, type DaySlots } from "@/components/provider/month-calendar";
import { CreateSlotDialog } from "@/components/provider/create-slot-dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/use-async-data";
import { cn } from "@/lib/utils";

function viewerTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return "UTC";
  }
}

function dateKeyInTz(iso: string, timezone: string): string {
  return isoToLocalInput(iso, timezone).slice(0, 10);
}

interface AvailabilityData {
  slots: Slot[];
  profile: ProviderProfile | null;
  /** Set when the profile lookup failed for a reason other than "no profile". */
  profileError: string | null;
}

export default function AvailabilityPage() {
  const slots = useAsyncData("slots", async (): Promise<AvailabilityData> => {
    const [slotsResult, profileSettled] = await Promise.all([
      listProviderSlots(),
      getMyProviderProfile().then(
        (profile) => ({ profile: profile as ProviderProfile | null, error: null as string | null }),
        (err) => {
          if (err instanceof ApiError && err.status === 404) {
            // No profile yet — an onboarding state, not a failure.
            return { profile: null, error: null };
          }
          return { profile: null, error: toErrorMessage(err) };
        },
      ),
    ]);
    return {
      slots: slotsResult.slots,
      profile: profileSettled.profile,
      profileError: profileSettled.error,
    };
  });

  const allSlots = useMemo(() => slots.data?.slots ?? [], [slots.data]);
  const profile = slots.data?.profile ?? null;
  // Business timezone comes only from the profile; without one, slot
  // creation is blocked (the browser timezone is never a business timezone).
  const timezone = profile?.timezone ?? "";

  // Initial calendar selection is "today" in the viewer's tz — a display
  // default only, never used for creating slots.
  const [selectedDate, setSelectedDate] = useState(() => dateKeyInTz(new Date().toISOString(), viewerTimezone()));
  const [createOpen, setCreateOpen] = useState(false);

  const profileMissing = !slots.loading && profile === null && !slots.data?.profileError;

  const days: DaySlots[] = useMemo(() => {
    if (timezone === "") return [];
    const map = new Map<string, { count: number; allClosed: boolean }>();
    for (const slot of allSlots) {
      const date = dateKeyInTz(slot.starts_at, timezone);
      const entry = map.get(date) ?? { count: 0, allClosed: true };
      entry.count += 1;
      if (slot.status === "open") entry.allClosed = false;
      map.set(date, entry);
    }
    return [...map.entries()].map(([date, entry]) => ({ date, ...entry }));
  }, [allSlots, timezone]);

  const daySlots = useMemo(
    () =>
      timezone === ""
        ? []
        : allSlots
            .filter((slot) => dateKeyInTz(slot.starts_at, timezone) === selectedDate)
            .sort((a, b) => a.starts_at.localeCompare(b.starts_at)),
    [allSlots, timezone, selectedDate],
  );

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageHeader
        title="Availability"
        description={
          profile
            ? `Times shown in your timezone: ${timezone} (${timeZoneAbbr(timezone)})`
            : "Complete your profile to publish availability."
        }
        actions={
          profile ? (
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <CalendarPlusIcon aria-hidden="true" />
              New slot
            </Button>
          ) : null
        }
      />

      {slots.error ? (
        <ErrorState
          icon={CalendarX2Icon}
          title="Couldn't load your availability"
          description={slots.error}
          action={
            <Button variant="outline" size="sm" onClick={slots.reload}>
              Retry
            </Button>
          }
        />
      ) : slots.data?.profileError ? (
        <ErrorState
          icon={CalendarX2Icon}
          title="Couldn't verify your profile"
          description={`${slots.data.profileError} Slot creation is disabled until your profile can be verified.`}
          action={
            <Button variant="outline" size="sm" onClick={slots.reload}>
              Retry
            </Button>
          }
        />
      ) : profileMissing ? (
        <EmptyState
          icon={UserRoundIcon}
          title="Finish your profile first"
          description="Customers can't see a provider without a display name and timezone. Create your profile — its timezone becomes your business timezone — and then come back to publish slots."
          action={
            <Button asChild size="sm">
              <Link href="/provider/profile">
                Complete your profile
                <ArrowRightIcon aria-hidden="true" />
              </Link>
            </Button>
          }
        />
      ) : slots.loading && allSlots.length === 0 ? (
        <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
          <Skeleton className="h-80 rounded-xl" />
          <Skeleton className="h-80 rounded-xl" />
        </div>
      ) : (
        <div className="grid items-start gap-4 lg:grid-cols-[320px_1fr]">
          <MonthCalendar
            days={days}
            selected={selectedDate}
            onSelect={setSelectedDate}
          />

          <div className="rounded-xl border bg-card">
            <div className="flex items-center justify-between border-b px-5 py-3.5">
              <h2 className="text-sm font-semibold">
                {new Date(`${selectedDate}T12:00:00`).toLocaleDateString("en-US", {
                  weekday: "long",
                  month: "long",
                  day: "numeric",
                })}
              </h2>
              <Button variant="outline" size="sm" onClick={() => setCreateOpen(true)}>
                <CalendarPlusIcon aria-hidden="true" />
                Add
              </Button>
            </div>

            {daySlots.length === 0 ? (
              <EmptyState
                className="border-0 bg-transparent"
                icon={CalendarPlusIcon}
                title="No slots on this day"
                description="Add availability so customers can book this day."
                action={
                  <Button variant="outline" size="sm" onClick={() => setCreateOpen(true)}>
                    Create a slot
                  </Button>
                }
              />
            ) : (
              <ul className="divide-y">
                {daySlots.map((slot) => (
                  <SlotRow key={slot.id} slot={slot} timezone={timezone} onChanged={slots.reload} />
                ))}
              </ul>
            )}
          </div>
        </div>
      )}

      {profile && createOpen ? (
        <CreateSlotDialog
          open={createOpen}
          onOpenChange={setCreateOpen}
          initial={{ date: selectedDate, startTime: "09:00", endTime: "10:00" }}
          timezone={timezone}
          onCreated={() => {
            slots.reload();
            setCreateOpen(false);
          }}
        />
      ) : null}
    </div>
  );
}

function SlotRow({ slot, timezone, onChanged }: { slot: Slot; timezone: string; onChanged: () => void }) {
  const [closing, setClosing] = useState(false);

  async function close() {
    setClosing(true);
    try {
      await closeProviderSlot(slot.id);
      toast.success("Slot closed", {
        description: "It no longer appears in public search.",
      });
      onChanged();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("Can't close this slot — it has an active booking.", {
          description: "Cancel or complete the booking first.",
        });
      } else {
        toast.error(toErrorMessage(err));
      }
    } finally {
      setClosing(false);
    }
  }

  return (
    <li className="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5">
      <div>
        <p className="text-sm font-medium">
          {formatTime(new Date(slot.starts_at), timezone)} –{" "}
          {formatTime(new Date(slot.ends_at), timezone)}
          <span className="ml-2 text-xs font-normal text-muted-foreground">
            {formatDuration(slot.starts_at, slot.ends_at)}
          </span>
        </p>
        <p className="text-xs text-muted-foreground">
          Created {new Date(slot.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
        </p>
      </div>
      <div className="flex items-center gap-2">
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
            slot.status === "open"
              ? "bg-emerald-50 text-emerald-700 ring-emerald-600/20"
              : "bg-neutral-100 text-neutral-600 ring-neutral-500/20",
          )}
        >
          <span
            className={cn(
              "size-1.5 rounded-full",
              slot.status === "open" ? "bg-emerald-500" : "bg-neutral-400",
            )}
            aria-hidden="true"
          />
          {slot.status === "open" ? "Open" : "Closed"}
        </span>
        {slot.status === "open" ? (
          <Button
            variant="outline"
            size="sm"
            onClick={() => void close()}
            disabled={closing}
          >
            {closing ? (
              <Loader2Icon className="animate-spin" aria-hidden="true" />
            ) : (
              <LockIcon aria-hidden="true" />
            )}
            Close
          </Button>
        ) : null}
      </div>
    </li>
  );
}
