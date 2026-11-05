"use client";

import { useState } from "react";
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  ClipboardListIcon,
  EyeIcon,
} from "lucide-react";import { toast } from "sonner";

import { listProviderBookings, updateProviderBookingStatus } from "@/lib/api/endpoints";
import type { ProviderBooking } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { bookingStatusMeta } from "@/lib/booking-status";
import { formatDuration, formatSlotRange } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { BookingDetailDialog } from "@/components/provider/booking-detail-dialog";
import { StatusMenu } from "@/components/provider/status-menu";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAsyncData } from "@/hooks/use-async-data";

const PAGE_SIZE = 10;

const statusTabs = [
  { value: "", label: "All" },
  { value: "confirmed", label: "Confirmed" },
  { value: "pending", label: "Pending" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
  { value: "no_show", label: "No-shows" },
];

interface Filters {
  status: string;
  from: string; // yyyy-mm-dd or ""
  to: string;
}

export default function ProviderBookingsPage() {
  const [filters, setFilters] = useState<Filters>({ status: "", from: "", to: "" });
  const [page, setPage] = useState(0);

  const list = useAsyncData(
    `${filters.status}#${filters.from}#${filters.to}#${page}`,
    () =>
      listProviderBookings({
        status: filters.status || undefined,
        from: filters.from ? new Date(`${filters.from}T00:00:00`).toISOString() : undefined,
        to: filters.to ? new Date(`${filters.to}T23:59:59`).toISOString() : undefined,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
  );

  const bookings = list.data?.bookings ?? [];
  const total = list.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const [detail, setDetail] = useState<ProviderBooking | null>(null);
  const [menuTarget, setMenuTarget] = useState<{
    booking: ProviderBooking;
    status: "confirmed" | "cancelled" | "completed" | "no_show";
  } | null>(null);
  const [transitioning, setTransitioning] = useState(false);

  async function confirmMenuTransition() {
    if (!menuTarget) return;
    setTransitioning(true);
    try {
      await updateProviderBookingStatus(menuTarget.booking.id, menuTarget.status);
      toast.success(`Booking marked as ${bookingStatusMeta(menuTarget.status).label.toLowerCase()}`);
      setMenuTarget(null);
      list.reload();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("That status change isn't allowed for this booking anymore.");
      } else {
        toast.error(toErrorMessage(err));
      }
      setMenuTarget(null);
    } finally {
      setTransitioning(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageHeader
        title="Bookings"
        description="Every appointment customers have made with you."
      />

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <Tabs
          value={filters.status}
          onValueChange={(value) => {
            setPage(0);
            setFilters((current) => ({ ...current, status: value }));
          }}
        >
          <TabsList className="max-w-full flex-wrap">
            {statusTabs.map((tab) => (
              <TabsTrigger key={tab.value} value={tab.value}>
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <div className="flex items-center gap-2">
          <Input
            type="date"
            aria-label="From date"
            className="w-36"
            value={filters.from}
            onChange={(event) => {
              setPage(0);
              setFilters((current) => ({ ...current, from: event.target.value }));
            }}
          />
          <span className="text-xs text-muted-foreground">to</span>
          <Input
            type="date"
            aria-label="To date"
            className="w-36"
            value={filters.to}
            onChange={(event) => {
              setPage(0);
              setFilters((current) => ({ ...current, to: event.target.value }));
            }}
          />
        </div>
      </div>

      {list.error ? (
        <ErrorState
          icon={ClipboardListIcon}
          title="Couldn't load bookings"
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
          icon={ClipboardListIcon}
          title="No bookings match"
          description="Adjust the filters, or publish more availability so customers can book."
        />
      ) : (
        <>
          <ul
            className={`divide-y rounded-xl border bg-card ${
              list.loading ? "opacity-60 transition-opacity" : ""
            }`}
          >
            {bookings.map((booking) => (
              <li key={booking.id} className="flex flex-wrap items-center gap-3 px-4 py-4 sm:px-5">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="truncate text-sm font-medium">{booking.customer_email}</p>
                    <StatusBadge status={booking.status} />
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {formatSlotRange(booking.slot_starts_at, booking.slot_ends_at)} ·{" "}
                    {formatDuration(booking.slot_starts_at, booking.slot_ends_at)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Button variant="ghost" size="sm" onClick={() => setDetail(booking)}>
                    <EyeIcon aria-hidden="true" />
                    Details
                  </Button>
                  <StatusMenu
                    booking={booking}
                    onSelect={(status) => setMenuTarget({ booking, status })}
                  />
                </div>
              </li>
            ))}
          </ul>

          {pageCount > 1 ? (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground" aria-live="polite">
                Page {page + 1} of {pageCount} · {total} bookings
              </p>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>
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

      <BookingDetailDialog
        booking={detail}
        onOpenChange={(open) => !open && setDetail(null)}
        onUpdated={list.reload}
      />

      <ConfirmDialog
        open={menuTarget !== null}
        onOpenChange={(open) => !open && setMenuTarget(null)}
        icon={ClipboardListIcon}
        title={
          menuTarget
            ? `Mark as ${bookingStatusMeta(menuTarget.status).label.toLowerCase()}?`
            : ""
        }
        description={
          menuTarget?.status === "cancelled"
            ? "The customer loses this appointment and the slot reopens for others."
            : "The customer's booking reflects the new status immediately."
        }
        confirmLabel={menuTarget ? bookingStatusMeta(menuTarget.status).label : "Confirm"}
        destructive={menuTarget?.status === "cancelled"}
        pending={transitioning}
        onConfirm={() => void confirmMenuTransition()}
      />
    </div>
  );
}
