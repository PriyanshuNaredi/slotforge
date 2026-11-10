"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import {
  CalendarSearchIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  SearchXIcon,
  UserRoundIcon,
} from "lucide-react";

import {
  searchAvailableSlots,
  getAvailableSlot,
  getPublicProvider,
} from "@/lib/api/endpoints";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SlotCard } from "@/components/customer/slot-card";
import { BookingDialog } from "@/components/customer/booking-dialog";
import { ProvidersDirectory } from "@/components/customer/providers-directory";
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
  const router = useRouter();
  const [rangeWindow, setRangeWindow] = useState<WindowValue>("14");
  const [page, setPage] = useState(0);

  // The active tab lives in the URL (?tab=providers|slots) so deep links
  // work without syncing props into state. ?provider=<id> implies slots.
  const providerId = searchParams.get("provider");
  const tab =
    searchParams.get("tab") === "providers" && !providerId
      ? "providers"
      : "slots";

  const switchTab = (value: string) => {
    const params = new URLSearchParams(searchParams);
    params.delete("provider");
    params.delete("slot");
    params.set("tab", value);
    router.replace(`/dashboard/discover?${params.toString()}`, { scroll: false });
  };

  const providerInfo = useAsyncData(
    tab === "slots" ? providerId : null,
    () => getPublicProvider(providerId!),
  );

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

  // Only fetch slots while the slots tab is visible.
  const list = useAsyncData(
    tab === "slots" ? `${rangeWindow}#${page}#${providerId ?? ""}` : null,
    () => {
      const range = rangeForWindow(rangeWindow);
      return searchAvailableSlots({
        provider_id: providerId ?? undefined,
        from: range.from,
        to: range.to,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      });
    },
  );

  const slots = list.data?.slots ?? [];
  const total = list.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageHeader
        title="Discover"
        description="Browse open slots, or find providers by name and availability."
      />

      <Tabs value={tab} onValueChange={switchTab}>
        <TabsList>
          <TabsTrigger value="slots">Available slots</TabsTrigger>
          <TabsTrigger value="providers">Providers</TabsTrigger>
        </TabsList>

        <TabsContent value="slots" className="space-y-6">
          <div className="flex justify-end">
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
          </div>

          {providerId ? (
            <div className="flex items-center justify-between gap-2 rounded-md border border-coral/30 bg-coral-wash/60 px-4 py-3 text-sm">
              <p className="flex min-w-0 items-center gap-2">
                <UserRoundIcon
                  className="size-4 shrink-0 text-brand-strong"
                  aria-hidden="true"
                />
                <span className="truncate">
                  Availability for{" "}
                  {providerInfo.data?.display_name ?? "a provider"}
                </span>
              </p>
              <Link
                href="/dashboard/discover"
                className="shrink-0 text-xs font-medium text-brand-strong underline-offset-4 hover:underline"
              >
                Show all slots
              </Link>
            </div>
          ) : null}

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
                <div key={index} className="rounded-lg border border-rule bg-card p-5">
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
                  <p className="text-sm text-ink-soft" aria-live="polite">
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
        </TabsContent>

        <TabsContent value="providers">
          <ProvidersDirectory />
        </TabsContent>
      </Tabs>
    </div>
  );
}
