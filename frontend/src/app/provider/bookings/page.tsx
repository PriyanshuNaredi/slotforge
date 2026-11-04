"use client";

import { useState } from "react";
import {
  CalendarRangeIcon,
  CheckCircle2Icon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CircleXIcon,
  ClipboardListIcon,
  EyeIcon,
  UserXIcon,
  type LucideIcon,
} from "lucide-react";
import { toast } from "sonner";

import { listProviderBookings, updateProviderBookingStatus } from "@/lib/api/endpoints";
import type { ProviderBooking } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { bookingStatusMeta, providerCanTransition } from "@/lib/booking-status";
import { formatDuration, formatSlotRange } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
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
  const [transitionTarget, setTransitionTarget] = useState<{
    booking: ProviderBooking;
    status: "confirmed" | "cancelled" | "completed" | "no_show";
  } | null>(null);
  const [transitioning, setTransitioning] = useState(false);

  async function confirmTransition() {
    if (!transitionTarget) return;
    setTransitioning(true);
    try {
      await updateProviderBookingStatus(transitionTarget.booking.id, transitionTarget.status);
      toast.success(`Booking marked as ${bookingStatusMeta(transitionTarget.status).label.toLowerCase()}`);
      setTransitionTarget(null);
      setDetail(null);
      list.reload();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("That status change isn't allowed for this booking anymore.");
      } else {
        toast.error(toErrorMessage(err));
      }
      setTransitionTarget(null);
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
                  {providerCanTransition(booking.status) ? (
                    <StatusMenu
                      booking={booking}
                      onSelect={(status) => setTransitionTarget({ booking, status })}
                    />
                  ) : null}
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

      {/* Detail dialog */}
      <Dialog open={detail !== null} onOpenChange={(open) => !open && setDetail(null)}>
        <DialogContent className="sm:max-w-lg">
          {detail ? (
            <>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2.5">
                  {detail.customer_email}
                  <StatusBadge status={detail.status} />
                </DialogTitle>
                <DialogDescription>
                  Booked {new Date(detail.created_at).toLocaleString("en-US", {
                    month: "short",
                    day: "numeric",
                    hour: "numeric",
                    minute: "2-digit",
                  })}
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4">
                <div className="rounded-lg border bg-background p-4">
                  <p className="flex items-center gap-2 text-sm font-medium">
                    <CalendarRangeIcon className="size-4 text-muted-foreground" aria-hidden="true" />
                    {formatSlotRange(detail.slot_starts_at, detail.slot_ends_at)}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {formatDuration(detail.slot_starts_at, detail.slot_ends_at)} · Slot #
                    {detail.slot_id.slice(0, 8)}
                  </p>
                </div>

                <div>
                  <p className="text-sm font-medium">Customer notes</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {detail.notes?.trim() ? detail.notes : "No notes added."}
                  </p>
                </div>

                <Separator />

                <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-sm">
                  <dt className="text-muted-foreground">Booking ID</dt>
                  <dd className="font-mono text-xs leading-6">{detail.id}</dd>
                  <dt className="text-muted-foreground">Customer ID</dt>
                  <dd className="font-mono text-xs leading-6">{detail.customer_id}</dd>
                  <dt className="text-muted-foreground">Last update</dt>
                  <dd>
                    {new Date(detail.updated_at).toLocaleString("en-US", {
                      month: "short",
                      day: "numeric",
                      hour: "numeric",
                      minute: "2-digit",
                    })}
                  </dd>
                </dl>
              </div>

              {providerCanTransition(detail.status) ? (
                <div className="flex flex-wrap justify-end gap-2">
                  {detail.status === "pending" ? (
                    <Button
                      size="sm"
                      variant="brand"
                      onClick={() => setTransitionTarget({ booking: detail, status: "confirmed" })}
                    >
                      <CheckCircle2Icon aria-hidden="true" />
                      Confirm
                    </Button>
                  ) : null}
                  {detail.status === "confirmed" ? (
                    <>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setTransitionTarget({ booking: detail, status: "completed" })}
                      >
                        <CheckCircle2Icon aria-hidden="true" />
                        Mark completed
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setTransitionTarget({ booking: detail, status: "no_show" })}
                      >
                        <UserXIcon aria-hidden="true" />
                        Mark no-show
                      </Button>
                    </>
                  ) : null}
                  <Button
                    size="sm"
                    variant="outline"
                    className="text-destructive hover:bg-destructive/5 hover:text-destructive"
                    onClick={() => setTransitionTarget({ booking: detail, status: "cancelled" })}
                  >
                    <CircleXIcon aria-hidden="true" />
                    Cancel booking
                  </Button>
                </div>
              ) : null}
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={transitionTarget !== null}
        onOpenChange={(open) => !open && setTransitionTarget(null)}
        icon={transitionTarget?.status === "cancelled" ? CircleXIcon : CheckCircle2Icon}
        title={
          transitionTarget
            ? `Mark as ${bookingStatusMeta(transitionTarget.status).label.toLowerCase()}?`
            : ""
        }
        description={
          transitionTarget?.status === "cancelled"
            ? "The customer loses this appointment and the slot reopens for others."
            : "The customer's booking reflects the new status immediately."
        }
        confirmLabel={transitionTarget ? bookingStatusMeta(transitionTarget.status).label : "Confirm"}
        destructive={transitionTarget?.status === "cancelled"}
        pending={transitioning}
        onConfirm={() => void confirmTransition()}
      />
    </div>
  );
}

function StatusMenu({
  booking,
  onSelect,
}: {
  booking: ProviderBooking;
  onSelect: (status: "confirmed" | "cancelled" | "completed" | "no_show") => void;
}) {
  // Mirrors the backend transition matrix (canTransitionBookingStatus):
  // pending -> confirmed|cancelled, confirmed -> completed|no_show|cancelled.
  const actions: {
    status: "confirmed" | "cancelled" | "completed" | "no_show";
    label: string;
    icon: LucideIcon;
    destructive?: boolean;
  }[] =
    booking.status === "pending"
      ? [
          { status: "confirmed", label: "Confirmed", icon: CheckCircle2Icon },
          { status: "cancelled", label: "Cancelled", icon: CircleXIcon, destructive: true },
        ]
      : [
          { status: "completed", label: "Completed", icon: CheckCircle2Icon },
          { status: "no_show", label: "No-show", icon: UserXIcon },
          { status: "cancelled", label: "Cancelled", icon: CircleXIcon, destructive: true },
        ];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          Update status
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Set status</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {actions.map((action) => (
          <DropdownMenuItem
            key={action.status}
            variant={action.destructive ? "destructive" : "default"}
            onClick={() => onSelect(action.status)}
          >
            <action.icon aria-hidden="true" />
            {action.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
