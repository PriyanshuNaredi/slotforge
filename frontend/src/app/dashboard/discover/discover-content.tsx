"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import {
  CalendarSearchIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  SearchXIcon,
} from "lucide-react";

import { searchAvailableSlots, getAvailableSlot } from "@/lib/api/endpoints";
import type { AvailableSlot } from "@/lib/api/types";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { SlotCard } from "@/components/customer/slot-card";
import { BookingDialog } from "@/components/customer/booking-dialog";
import { useAsyncData } from "@/hooks/use-async-data";

const PAGE_SIZE = 12;

const windows = [
  { value: "7", label: "Next 7 days" },
  { value: "14", label: "Next 14 days" },
  { value: "30", label: "Next 30 days" },
  { value: "all", label: "Any time" },
] as const;

type WindowValue = (typeof windows)[number]["value"];

function rangeForWindow(value: WindowValue): { from: string; to?: string } {
  const now = new Date();
  if (value === "all") return { from: now.toISOString() };
  const to = new Date(now);
  to.setDate(to.getDate() + Number(value));
  return { from: now.toISOString(), to: to.toISOString() };
}

export function DiscoverContent() {
  const searchParams = useSearchParams();
  const [rangeWindow, setRangeWindow] = useState<WindowValue>("14");
  const [page, setPage] = useState(0);

  const list = useAsyncData(`${rangeWindow}#${page}`, () => {
    const range = rangeForWindow(rangeWindow);
    return searchAvailableSlots({
      from: range.from,
      to: range.to,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
    });
  });

  const slots = list.data?.slots ?? [];
  const total = list.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Deep link: /dashboard/discover?slot=<id> opens the booking dialog.
  const deepLinkHandled = useRef(false);
  const [bookingSlot, setBookingSlot] = useState<AvailableSlot | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    if (deepLinkHandled.current) return;
    const slotId = searchParams.get("slot");
    if (!slotId) return;
    deepLinkHandled.current = true;
    getAvailableSlot(slotId).then(
      (slot) => {
        setBookingSlot(slot);
        setDialogOpen(true);
      },
      () => {
        // Slot gone (booked/closed) — the grid shows current reality.
      },
    );
  }, [searchParams]);

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageHeader
        title="Discover availability"
        description="Real open slots from providers on SlotForge. Booking confirms instantly."
        actions={
          <Select
            value={rangeWindow}
            onValueChange={(value) => {
              setPage(0);
              setRangeWindow(value as WindowValue);
            }}
          >
            <SelectTrigger aria-label="Date range" className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {windows.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        }
      />

      {list.error ? (
        <ErrorState
          icon={SearchXIcon}
          title="Couldn't load availability"
          description={list.error}
          action={
            <Button variant="outline" size="sm" onClick={list.reload}>
              Try again
            </Button>
          }
        />
      ) : list.loading && slots.length === 0 ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <div key={index} className="rounded-xl border bg-card p-5">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="mt-3 h-3 w-44" />
              <Skeleton className="mt-5 h-8 w-full" />
            </div>
          ))}
        </div>
      ) : !list.loading && slots.length === 0 ? (
        <EmptyState
          icon={CalendarSearchIcon}
          title="No open slots in this window"
          description="Try a wider date range, or check back soon — new availability appears the moment providers publish it."
        />
      ) : (
        <>
          <div className={`grid gap-4 sm:grid-cols-2 lg:grid-cols-3 ${list.loading ? "opacity-60 transition-opacity" : ""}`}>
            {slots.map((slot) => (
              <SlotCard
                key={slot.id}
                slot={slot}
                onBook={(selected) => {
                  setBookingSlot(selected);
                  setDialogOpen(true);
                }}
              />
            ))}
          </div>

          {pageCount > 1 ? (
            <div className="flex items-center justify-between pt-2">
              <p className="text-sm text-muted-foreground" aria-live="polite">
                Page {page + 1} of {pageCount} · {total} slots
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 0}
                  onClick={() => setPage((value) => value - 1)}
                >
                  <ChevronLeftIcon aria-hidden="true" />
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= pageCount - 1}
                  onClick={() => setPage((value) => value + 1)}
                >
                  Next
                  <ChevronRightIcon aria-hidden="true" />
                </Button>
              </div>
            </div>
          ) : null}
        </>
      )}

      <BookingDialog
        slot={bookingSlot}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onBooked={list.reload}
      />
    </div>
  );
}
