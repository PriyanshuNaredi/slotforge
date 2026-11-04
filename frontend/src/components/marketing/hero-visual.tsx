"use client";

import { motion, useReducedMotion } from "motion/react";
import { CalendarCheckIcon, CheckIcon, ClockIcon } from "lucide-react";

/**
 * Product preview for the hero: a stylized availability list and the
 * confirmation state that follows a booking. Static content, real styling.
 */
export function HeroVisual() {
  const reduceMotion = useReducedMotion();
  const fadeUp = reduceMotion
    ? {}
    : {
        initial: { opacity: 0, y: 16 },
        animate: { opacity: 1, y: 0 },
      };

  return (
    <div className="relative mx-auto w-full max-w-md" aria-hidden="true">
      <motion.div
        {...fadeUp}
        transition={{ duration: 0.5, ease: "easeOut" }}
        className="rounded-xl border bg-card p-4 shadow-xl shadow-black/[0.06]"
      >
        <div className="flex items-center justify-between border-b border-border/70 pb-3">
          <div>
            <p className="text-[13px] font-semibold">Dr. Amara Marsh</p>
            <p className="text-xs text-muted-foreground">Career consultation · 45 min</p>
          </div>
          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 ring-1 ring-inset ring-emerald-600/20">
            <span className="size-1.5 rounded-full bg-emerald-500" />
            3 open slots
          </span>
        </div>

        <ul className="mt-3 space-y-2">
          {[
            { day: "Tue, Nov 5", time: "9:00 – 9:45 AM", state: "open" },
            { day: "Tue, Nov 5", time: "10:30 – 11:15 AM", state: "booked" },
            { day: "Wed, Nov 6", time: "2:00 – 2:45 PM", state: "open" },
          ].map((slot) => (
            <li
              key={slot.time}
              className={`flex items-center justify-between rounded-lg border px-3 py-2.5 ${
                slot.state === "booked"
                  ? "border-border/60 bg-muted/50 opacity-60"
                  : "border-border bg-background"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <ClockIcon className="size-4 text-muted-foreground" />
                <div>
                  <p className="text-[13px] font-medium">{slot.day}</p>
                  <p className="text-xs text-muted-foreground">{slot.time}</p>
                </div>
              </div>
              {slot.state === "open" ? (
                <span className="rounded-md bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground">
                  Book
                </span>
              ) : (
                <span className="text-xs font-medium text-muted-foreground">Booked</span>
              )}
            </li>
          ))}
        </ul>
      </motion.div>

      <motion.div
        {...fadeUp}
        transition={{ duration: 0.5, delay: 0.25, ease: "easeOut" }}
        className="absolute -bottom-6 -left-4 flex items-center gap-3 rounded-xl border bg-card px-4 py-3 shadow-xl shadow-black/[0.08] sm:-left-10"
      >
        <span className="flex size-8 items-center justify-center rounded-full bg-emerald-500/15">
          <CheckIcon className="size-4 text-emerald-600" />
        </span>
        <div>
          <p className="text-[13px] font-semibold leading-tight">Booking confirmed</p>
          <p className="text-xs text-muted-foreground">Tue, Nov 5 · 9:00 AM · CST</p>
        </div>
      </motion.div>

      <motion.div
        {...fadeUp}
        transition={{ duration: 0.5, delay: 0.4, ease: "easeOut" }}
        className="absolute -top-5 -right-3 hidden items-center gap-2.5 rounded-xl border bg-card px-3.5 py-2.5 shadow-xl shadow-black/[0.08] sm:flex"
      >
        <CalendarCheckIcon className="size-4 text-brand" />
        <p className="text-xs font-medium">Added to your calendar</p>
      </motion.div>

      <div className="absolute -right-6 -bottom-8 hidden size-24 rounded-full bg-brand-soft blur-2xl sm:block" />
      <div className="absolute -left-10 -top-10 hidden size-32 rounded-full bg-brand-soft/70 blur-2xl sm:block" />
    </div>
  );
}
