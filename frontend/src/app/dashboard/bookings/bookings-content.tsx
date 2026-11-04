"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import {
  CalendarClockIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CalendarX2Icon,
} from "lucide-react";

import { cancelBooking, listCustomerBookings } from "@/lib/api/endpoints";
import type { Booking, BookingStatus } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { customerCanCancel } from "@/lib/booking-status";
import { formatRelative, formatSlotRange } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useSlotDetails } from "@/hooks/use-slot-details";
import { useAsyncData } from "@/hooks/use-async-data";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";

const PAGE_SIZE = 10;

const tabs = [
  { value: "", label: "All" },
  { value: "confirmed", label: "Upcoming" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
  { value: "no_show", label: "No-shows" },
];

export function CustomerBookingsContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const statusFilter = searchParams.get("status") ?? "";
  const page = Number(searchParams.get("page") ?? "0") || 0;

  const list = useAsyncData(`${statusFilter}#${page}`, () =>
    listCustomerBookings({
      status: (statusFilter || undefined) as BookingStatus | undefined,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
    }),
  );

  const bookings = list.data?.bookings ?? [];
  const total = list.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const { slots } = useSlotDetails(bookings.map((booking) => booking.slot_id));

  const [cancelTarget, setCancelTarget] = useState<Booking | null>(null);
  const [cancelling, setCancelling] = useState(false);

  function setPage(next: number) {
    const params = new URLSearchParams(searchParams);
    if (next > 0) params.set("page", String(next));
    else params.delete("page");
    router.replace(`/dashboard/bookings${params.toString() ? `?${params}` : ""}`);
  }

  async function confirmCancel() {
    if (!cancelTarget) return;
    setCancelling(true);
    try {
      await cancelBooking(cancelTarget.id);
      toast.success("Booking cancelled", {
        description: "The slot is open for others again.",
      });
      setCancelTarget(null);
      list.reload();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("This booking can no longer be cancelled.");
      } else {
        toast.error(toErrorMessage(err));
      }
      setCancelTarget(null);
    } finally {
      setCancelling(false);
    }
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <PageHeader
        title="My bookings"
        description="Everything you've booked, from upcoming appointments to history."
      />

      <Tabs
        value={statusFilter}
        onValueChange={(value) => {
          const params = new URLSearchParams();
          if (value) params.set("status", value);
          router.replace(`/dashboard/bookings${params.toString() ? `?${params}` : ""}`);
        }}
      >
        <TabsList className="max-w-full flex-wrap">
          {tabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {list.error ? (
        <ErrorState
          icon={CalendarX2Icon}
          title="Couldn't load your bookings"
          description={list.error}
          action={
            <Button variant="outline" size="sm" onClick={list.reload}>
              Try again
            </Button>
          }
        />
      ) : list.loading && bookings.length === 0 ? (
        <div className="space-y-3">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      ) : !list.loading && bookings.length === 0 ? (
        <EmptyState
          icon={CalendarClockIcon}
          title={statusFilter === "" ? "No bookings yet" : "No bookings with this status"}
          description={
            statusFilter === ""
              ? "When you book a slot it appears here with its full history."
              : "Try a different filter, or browse open availability."
          }
          action={
            <Button asChild size="sm">
              <Link href="/dashboard/discover">Browse availability</Link>
            </Button>
          }
        />
      ) : (
        <>
          <ul
            className={`divide-y rounded-xl border bg-card ${
              list.loading ? "opacity-60 transition-opacity" : ""
            }`}
          >
            {bookings.map((booking) => {
              const slot = slots[booking.slot_id];
              return (
                <li key={booking.id} className="flex flex-wrap items-center gap-3 px-4 py-4 sm:px-5">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        href={`/dashboard/bookings/${booking.id}`}
                        className="text-sm font-medium underline-offset-4 hover:underline"
                      >
                        {slot?.provider_name ?? "Booking"}
                      </Link>
                      <StatusBadge status={booking.status} />
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {slot
                        ? formatSlotRange(slot.starts_at, slot.ends_at, slot.timezone)
                        : `Booked ${formatRelative(new Date(booking.created_at))}`}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button asChild variant="ghost" size="sm">
                      <Link href={`/dashboard/bookings/${booking.id}`}>Details</Link>
                    </Button>
                    {customerCanCancel(booking.status) ? (
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-destructive hover:bg-destructive/5 hover:text-destructive"
                        onClick={() => setCancelTarget(booking)}
                      >
                        Cancel
                      </Button>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>

          {pageCount > 1 ? (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground" aria-live="polite">
                Page {page + 1} of {pageCount} · {total} bookings
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 0}
                  onClick={() => setPage(page - 1)}
                >
                  <ChevronLeftIcon aria-hidden="true" />
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= pageCount - 1}
                  onClick={() => setPage(page + 1)}
                >
                  Next
                  <ChevronRightIcon aria-hidden="true" />
                </Button>
              </div>
            </div>
          ) : null}
        </>
      )}

      <ConfirmDialog
        open={cancelTarget !== null}
        onOpenChange={(open) => !open && setCancelTarget(null)}
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
