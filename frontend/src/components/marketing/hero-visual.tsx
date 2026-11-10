"use client";

import { CalendarCheckIcon, CheckIcon, ClockIcon } from "lucide-react";

/**
 * Product preview for the hero: a stylized availability list and the
 * confirmation state that follows a booking. Static, illustrative content —
 * the live feed further down the page is the real thing.
 *
 * Layering rules (learned from review): the soft color blobs render BEFORE
 * the card so they stay behind it, and the floating toast/badge only overlap
 * the card's padding zones — never a text row.
 */
export function HeroVisual() {
  return (
    <div className="relative mx-auto w-full max-w-md" aria-hidden="true">
      {/* Soft depth blobs — behind the card, overlapping edges only. */}
      <div className="absolute -left-14 -top-12 hidden size-36 rounded-full bg-coral/25 blur-3xl sm:block" />
      <div className="absolute -bottom-10 -right-8 hidden size-40 rounded-full bg-navy/15 blur-3xl sm:block" />

      {/* The product card */}
      <div className="animate-fade-up relative rounded-2xl border border-border bg-card p-4 shadow-xl shadow-navy/10">
        <div className="flex items-center justify-between gap-3 border-b border-border/70 pb-3">
          <div className="min-w-0">
            <p className="truncate text-[13px] font-semibold">Dr. Amara Marsh</p>
            <p className="text-xs text-ink-soft">Career consultation · 45 min</p>
          </div>
          <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-success-soft px-2 py-0.5 text-xs font-medium text-success ring-1 ring-inset ring-success/20">
            <span className="size-1.5 rounded-full bg-success" />
            3 open slots
          </span>
        </div>

        <ul className="mt-3 space-y-2">
          {[
            { day: "Tue, Nov 5", time: "9:00 – 9:45 AM", state: "open" },
            { day: "Tue, Nov 5", time: "10:30 – 11:15 AM", state: "booked" },
            { day: "Wed, Nov 6", time: "2:00 – 2:45 PM", state: "open" },
          ].map((slot, index) => (
            <li
              key={slot.time}
              style={{ animationDelay: `${180 + index * 90}ms` }}
              className={`animate-fade-up flex items-center justify-between rounded-lg border px-3 py-2.5 ${
                slot.state === "booked"
                  ? "border-border/60 bg-muted/50 opacity-60"
                  : "border-border bg-background"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <ClockIcon className="size-4 text-ink-soft" />
                <div>
                  <p className="text-[13px] font-medium">{slot.day}</p>
                  <p className="text-xs text-ink-soft">{slot.time}</p>
                </div>
              </div>
              {slot.state === "open" ? (
                <span className="rounded-md bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground">
                  Book
                </span>
              ) : (
                <span className="text-xs font-medium text-ink-soft">Booked</span>
              )}
            </li>
          ))}
        </ul>
      </div>

      {/* Floating confirmation toast — overlaps the card's bottom padding
          zone only (never the last row's text). */}
      <div className="animate-pop absolute -bottom-9 -left-3 [animation-delay:350ms] sm:-left-8">
        <div className="animate-float flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3 shadow-xl shadow-navy/10">
          <span className="flex size-8 items-center justify-center rounded-full bg-success-soft">
            <CheckIcon className="size-4 text-success" />
          </span>
          <div>
            <p className="text-[13px] font-semibold leading-tight">Booking confirmed</p>
            <p className="text-xs text-ink-soft">Tue, Nov 5 · 9:00 AM · CST</p>
          </div>
        </div>
      </div>

      {/* Calendar-added badge — above the header's padding zone. */}
      <div className="animate-pop absolute -top-7 -right-2 hidden [animation-delay:550ms] sm:block">
        <div className="animate-float-late flex items-center gap-2.5 rounded-xl border border-border bg-card px-3.5 py-2.5 shadow-xl shadow-navy/10">
          <CalendarCheckIcon className="size-4 text-brand-strong" />
          <p className="text-xs font-medium">Added to your calendar</p>
        </div>
      </div>
    </div>
  );
}
