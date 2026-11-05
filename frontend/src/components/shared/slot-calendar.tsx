"use client";

import { useMemo } from "react";
import FullCalendar from "@fullcalendar/react";
import type { EventClickInfo, EventInput, DateSelectInfo } from "@fullcalendar/react";
import dayGridPlugin from "@fullcalendar/react/daygrid";
import timeGridPlugin from "@fullcalendar/react/timegrid";
import listPlugin from "@fullcalendar/react/list";
import interactionPlugin from "@fullcalendar/react/interaction";
import classicThemePlugin from "@fullcalendar/react/themes/classic";
import type { LucideIcon } from "lucide-react";
import { CalendarSearchIcon, CalendarX2Icon } from "lucide-react";

import "@fullcalendar/react/skeleton.css";
import "@fullcalendar/react/themes/classic/theme.css";
import "@fullcalendar/react/themes/classic/palette.css";

import { EmptyState, ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useIsMobile } from "@/hooks/use-is-mobile";
import { viewerTimezone } from "@/lib/format";

export interface SlotCalendarProps {
  events: EventInput[];
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  emptyIcon?: LucideIcon;
  emptyTitle: string;
  emptyDescription: string;
  /** IANA timezone used for display; omit for the viewer's local time. */
  timeZone?: string;
  /** Desktop default view; mobile always starts on the agenda list. */
  desktopInitialView?: "dayGridMonth" | "timeGridWeek" | "timeGridDay";
  selectable?: boolean;
  onDateSelect?: (selection: { start: string; end: string }) => void;
  onEventClick?: (eventId: string, kind: string | undefined) => void;
  /** Fires whenever the visible date range or view changes. */
  onDatesSet?: (info: { view: string; start: string; end: string }) => void;
  ariaLabel?: string;
}

/**
 * Shared FullCalendar wrapper for customer and provider experiences:
 * month/week/day + agenda views, timezone-aware display, mobile agenda
 * default, and slotforge-skinned styling (see globals.css).
 */
export function SlotCalendar({
  events,
  loading = false,
  error = null,
  onRetry,
  emptyIcon: EmptyIcon = CalendarSearchIcon,
  emptyTitle,
  emptyDescription,
  timeZone,
  desktopInitialView = "dayGridMonth",
  selectable = false,
  onDateSelect,
  onEventClick,
  onDatesSet,
  ariaLabel = "Calendar",
}: SlotCalendarProps) {
  const isMobile = useIsMobile();

  // FullCalendar v7 (Temporal-based) requires a concrete IANA timezone.
  const resolvedTimeZone = timeZone ?? viewerTimezone();

  const plugins = useMemo(
    () => [classicThemePlugin, dayGridPlugin, timeGridPlugin, listPlugin, interactionPlugin],
    [],
  );

  if (error) {
    return (
      <ErrorState
        icon={CalendarX2Icon}
        title="Couldn't load the calendar"
        description={error}
        action={
          onRetry ? (
            <Button variant="outline" size="sm" onClick={onRetry}>
              Try again
            </Button>
          ) : null
        }
      />
    );
  }

  if (loading && events.length === 0) {
    return <Skeleton className="h-96 w-full rounded-xl" />;
  }

  if (!loading && events.length === 0) {
    return (
      <EmptyState
        icon={EmptyIcon}
        title={emptyTitle}
        description={emptyDescription}
      />
    );
  }

  function handleSelect(info: DateSelectInfo) {
    info.jsEvent?.preventDefault();
    onDateSelect?.({ start: info.startStr, end: info.endStr });
  }

  function handleEventClick(info: EventClickInfo) {
    info.jsEvent?.preventDefault();
    const kind = (info.event.extendedProps as { kind?: string } | undefined)?.kind;
    onEventClick?.(info.event.id, kind);
  }

  return (
    <div
      className={`fc-wrapper ${loading ? "opacity-60 transition-opacity" : ""}`}
      aria-label={ariaLabel}
    >
      <FullCalendar
        plugins={plugins}
        initialView={isMobile ? "listWeek" : desktopInitialView}
        timeZone={resolvedTimeZone}
        headerToolbar={{
          left: "prev,next today",
          center: "title",
          right: "dayGridMonth,timeGridWeek,timeGridDay,listWeek",
        }}
        events={events}
        selectable={selectable}
        selectMirror
        unselectAuto
        select={handleSelect}
        eventClick={handleEventClick}
        datesSet={(info) =>
          onDatesSet?.({ view: info.view.type, start: info.startStr, end: info.endStr })
        }
        height="auto"
        expandRows
        allDaySlot={false}
        dayMaxEventRows={4}
        nowIndicator
        dayHeaderFormat={{ weekday: "short" }}
      />
    </div>
  );
}
