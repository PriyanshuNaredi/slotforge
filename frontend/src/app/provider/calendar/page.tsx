"use client";

import Link from "next/link";
import { useState } from "react";
import {
  ArrowRightIcon,
  CalendarDaysIcon,
  CalendarPlusIcon,
  CalendarX2Icon,
  LockIcon,
  UserRoundIcon,
} from "lucide-react";
import { toast } from "sonner";

import {
  closeProviderSlot,
  getMyProviderProfile,
  listProviderBookings,
  listProviderSlots,
} from "@/lib/api/endpoints";
import type { ProviderBooking, ProviderProfile, Slot } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { isoToLocalInput, selectionToSlotPrefill, timeZoneAbbr } from "@/lib/format";
import { providerBookingEvents, providerSlotEvents } from "@/lib/calendar-events";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { SlotCalendar } from "@/components/shared/slot-calendar";
import { BookingDetailDialog } from "@/components/provider/booking-detail-dialog";
import { CreateSlotDialog } from "@/components/provider/create-slot-dialog";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useAsyncData } from "@/hooks/use-async-data";
import { cn } from "@/lib/utils";

interface CalendarData {
  slots: Slot[];
  bookings: ProviderBooking[];
  profile: ProviderProfile | null;
}

/** Provider calendar: open slots, bookings, drag-to-publish. */
export default function ProviderCalendarPage() {
  const data = useAsyncData("provider-calendar", async (): Promise<CalendarData> => {
    const [slotsResult, bookingsResult] = await Promise.all([
      listProviderSlots(),
      listProviderBookings({ limit: 100 }),
    ]);

    let profile: ProviderProfile | null = null;
    try {
      profile = await getMyProviderProfile();
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 404)) throw err;
    }

    return {
      slots: slotsResult.slots,
      bookings: bookingsResult.bookings,
      profile,
    };
  });

  const slots = data.data?.slots ?? [];
  const bookings = data.data?.bookings ?? [];
  const profile = data.data?.profile ?? null;
  const timezone = profile?.timezone ?? "";

  const events = [
    ...providerSlotEvents(slots),
    ...providerBookingEvents(bookings),
  ];

  const eventsLoading = data.loading;

  // Slot detail dialog
  const [slotDetail, setSlotDetail] = useState<Slot | null>(null);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const [closing, setClosing] = useState(false);

  // Booking detail dialog
  const [bookingDetail, setBookingDetail] = useState<ProviderBooking | null>(null);

  // Create-slot dialog, prefilled from a calendar selection
  const [createOpen, setCreateOpen] = useState(false);
  const [createInitial, setCreateInitial] = useState({ date: "", startTime: "09:00", endTime: "10:00" });

  function handleEventClick(eventId: string, kind: string | undefined) {
    if (kind === "slot") {
      const slot = slots.find((candidate) => candidate.id === eventId.slice("slot:".length));
      if (slot) setSlotDetail(slot);
    } else if (kind === "booking") {
      const booking = bookings.find((candidate) => candidate.id === eventId.slice("booking:".length));
      if (booking) setBookingDetail(booking);
    }
  }

  function handleDateSelect(selection: { start: string; end: string }) {
    if (!timezone) return;
    setCreateInitial(selectionToSlotPrefill(selection.start, selection.end, timezone));
    setCreateOpen(true);
  }

  async function confirmCloseSlot() {
    if (!slotDetail) return;
    setClosing(true);
    try {
      await closeProviderSlot(slotDetail.id);
      toast.success("Slot closed", {
        description: "It no longer appears in public search.",
      });
      setCloseConfirmOpen(false);
      setSlotDetail(null);
      data.reload();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("Can't close this slot — it has an active booking.", {
          description: "Cancel or complete the booking first.",
        });
      } else {
        toast.error(toErrorMessage(err));
      }
      setCloseConfirmOpen(false);
    } finally {
      setClosing(false);
    }
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <PageHeader
        title="Calendar"
        description={
          profile
            ? `Availability and bookings in your timezone: ${timezone} (${timeZoneAbbr(timezone)}) · publish by dragging on the grid, or with “New slot”`
            : "Complete your profile to publish availability."
        }
        actions={
          profile ? (
            <Button
              size="sm"
              onClick={() => {
                setCreateInitial({ date: isoToLocalInput(new Date().toISOString(), timezone).slice(0, 10), startTime: "09:00", endTime: "10:00" });
                setCreateOpen(true);
              }}
            >
              <CalendarPlusIcon aria-hidden="true" />
              New slot
            </Button>
          ) : null
        }
      />

      {data.error ? (
        <ErrorState
          icon={CalendarX2Icon}
          title="Couldn't load your calendar"
          description={data.error}
          action={
            <Button variant="outline" size="sm" onClick={data.reload}>
              Retry
            </Button>
          }
        />
      ) : !data.loading && profile === null ? (
        <EmptyState
          icon={UserRoundIcon}
          title="Finish your profile first"
          description="Create your provider profile — its timezone drives your calendar — then come back to schedule availability."
          action={
            <Button asChild size="sm">
              <Link href="/provider/profile">
                Complete your profile
                <ArrowRightIcon aria-hidden="true" />
              </Link>
            </Button>
          }
        />
      ) : (
        <div className="rounded-lg border border-rule bg-card p-4">
          <SlotCalendar
            events={events}
            loading={eventsLoading}
            error={null}
            timeZone={timezone || undefined}
            desktopInitialView="timeGridWeek"
            selectable
            onDateSelect={handleDateSelect}
            onEventClick={handleEventClick}
            emptyIcon={CalendarDaysIcon}
            emptyTitle="Nothing scheduled"
            emptyDescription="Publish availability slots and confirmed bookings will appear here."
            ariaLabel="Provider calendar"
          />
        </div>
      )}

      {/* Slot detail */}
      <Dialog open={slotDetail !== null} onOpenChange={(open) => !open && setSlotDetail(null)}>
        <DialogContent className="sm:max-w-md">
          {slotDetail ? (
            <>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2.5">
                  {slotDetail.status === "open" ? "Open slot" : "Closed slot"}
                  <span
                    className={cn(
                      "inline-flex items-center gap-1.5 rounded-sm px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ring-1 ring-inset",
                      slotDetail.status === "open"
                        ? "bg-success-soft text-success ring-success/25"
                        : "bg-muted text-ink-soft ring-rule-strong",
                    )}
                  >
                    <span
                      className={cn(
                        "size-1.5 rounded-[1px]",
                        slotDetail.status === "open" ? "bg-success" : "bg-rule-strong",
                      )}
                      aria-hidden="true"
                    />
                    {slotDetail.status === "open" ? "Open" : "Closed"}
                  </span>
                </DialogTitle>
                <DialogDescription>
                  {new Date(slotDetail.starts_at).toLocaleString("en-US", {
                    weekday: "long",
                    month: "long",
                    day: "numeric",
                    hour: "numeric",
                    minute: "2-digit",
                    timeZone: timezone,
                  })}{" "}
                  –{" "}
                  {new Date(slotDetail.ends_at).toLocaleTimeString("en-US", {
                    hour: "numeric",
                    minute: "2-digit",
                    timeZone: timezone,
                  })}{" "}
                  ({timeZoneAbbr(timezone)})
                </DialogDescription>
              </DialogHeader>
              <p className="text-sm text-ink-soft">
                {slotDetail.status === "open"
                  ? "Customers can book this slot from public search."
                  : "This slot no longer appears in public search."}
              </p>
              {slotDetail.status === "open" ? (
                <div className="flex justify-end">
                  <Button
                    variant="outline"
                    onClick={() => setCloseConfirmOpen(true)}
                  >
                    <LockIcon aria-hidden="true" />
                    Close slot
                  </Button>
                </div>
              ) : null}
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={closeConfirmOpen}
        onOpenChange={setCloseConfirmOpen}
        icon={LockIcon}
        title="Close this slot?"
        description="It will be removed from public search. Slots with an active booking can't be closed."
        confirmLabel="Close slot"
        pending={closing}
        onConfirm={() => void confirmCloseSlot()}
      />

      <BookingDetailDialog
        booking={bookingDetail}
        onOpenChange={(open) => !open && setBookingDetail(null)}
        onUpdated={data.reload}
      />

      {profile && createOpen ? (
        <CreateSlotDialog
          key={`${createInitial.date}T${createInitial.startTime}`}
          open={createOpen}
          onOpenChange={setCreateOpen}
          initial={createInitial}
          timezone={timezone}
          onCreated={() => {
            data.reload();
            setCreateOpen(false);
          }}
        />
      ) : null}
    </div>
  );
}
