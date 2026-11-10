"use client";

import { useMemo, useState } from "react";
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface DaySlots {
  /** yyyy-mm-dd in the provider's timezone. */
  date: string;
  count: number;
  allClosed: boolean;
}

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function todayIso(): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
  return parts;
}

/**
 * Month calendar for availability browsing. Days with slots show a dot;
 * days with only closed slots show a muted dot.
 */
export function MonthCalendar({
  days,
  selected,
  onSelect,
  className,
}: {
  days: DaySlots[];
  selected: string;
  onSelect: (date: string) => void;
  className?: string;
}) {
  const [view, setView] = useState(() => {
    const [year, month] = selected.split("-").map(Number);
    return { year, month };
  });

  const byDate = useMemo(
    () => new Map(days.map((day) => [day.date, day])),
    [days],
  );

  const grid = useMemo(() => {
    const first = new Date(view.year, view.month - 1, 1);
    const startOffset = first.getDay(); // Sunday-first
    const daysInMonth = new Date(view.year, view.month, 0).getDate();

    const cells: (string | null)[] = [];
    for (let i = 0; i < startOffset; i++) cells.push(null);
    for (let day = 1; day <= daysInMonth; day++) {
      cells.push(
        `${view.year}-${String(view.month).padStart(2, "0")}-${String(day).padStart(2, "0")}`,
      );
    }
    while (cells.length % 7 !== 0) cells.push(null);
    return cells;
  }, [view]);

  const monthLabel = new Date(view.year, view.month - 1, 1).toLocaleDateString(
    "en-US",
    { month: "long", year: "numeric" },
  );

  function shiftMonth(delta: number) {
    setView((current) => {
      const next = new Date(current.year, current.month - 1 + delta, 1);
      return { year: next.getFullYear(), month: next.getMonth() + 1 };
    });
  }

  const today = todayIso();

  return (
    <div className={cn("rounded-lg border border-rule bg-card p-4", className)}>
      <div className="flex items-center justify-between">
        <h2 className="font-display text-base font-semibold tracking-tight" aria-live="polite">
          {monthLabel}
        </h2>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Previous month"
            onClick={() => shiftMonth(-1)}
          >
            <ChevronLeftIcon className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Next month"
            onClick={() => shiftMonth(1)}
          >
            <ChevronRightIcon className="size-4" />
          </Button>
        </div>
      </div>

      <div className="mt-3 grid grid-cols-7 gap-1 text-center text-[11px] font-medium text-ink-soft">
        {WEEKDAYS.map((weekday) => (
          <div key={weekday} className="py-1">
            {weekday}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-7 gap-1">
        {grid.map((date, index) => {
          if (date === null) return <div key={`empty-${index}`} />;

          const dayInfo = byDate.get(date);
          const dayNumber = Number(date.slice(8, 10));
          const isSelected = date === selected;
          const isToday = date === today;
          // Past days read dimmer so bookable days stand out at a glance.
          const isPast = date < today;

          return (
            <button
              key={date}
              type="button"
              aria-pressed={isSelected}
              aria-label={`${new Date(`${date}T12:00:00`).toLocaleDateString("en-US", { weekday: "long", month: "long", day: "numeric" })}${dayInfo ? `, ${dayInfo.count} slot${dayInfo.count === 1 ? "" : "s"}` : ""}`}
              onClick={() => onSelect(date)}
              className={cn(
                "relative flex h-11 flex-col items-center justify-center rounded-sm text-sm transition-colors cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-coral",
                isSelected
                  ? "bg-primary text-primary-foreground font-semibold"
                  : dayInfo
                    ? "bg-coral-wash/70 text-ink hover:bg-coral-wash"
                    : "hover:bg-accent",
                !isSelected && isToday && "ring-1 ring-inset ring-coral/60",
                !isSelected && isPast && "opacity-55",
              )}
            >
              {dayNumber}
              {dayInfo ? (
                <span
                  aria-hidden="true"
                  className={cn(
                    "absolute bottom-1.5 size-1 rounded-full",
                    isSelected
                      ? "bg-primary-foreground"
                      : dayInfo.allClosed
                        ? "bg-rule-strong"
                        : "bg-coral",
                  )}
                />
              ) : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}
