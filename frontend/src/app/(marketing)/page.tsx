import Link from "next/link";
import {
  BellRingIcon,
  CalendarClockIcon,
  CalendarRangeIcon,
  LockIcon,
  SearchIcon,
  UserPlusIcon,
  ZapIcon,
} from "lucide-react";

import { HeroVisual } from "@/components/marketing/hero-visual";
import { AvailabilityPreview } from "@/components/marketing/availability-preview";
import { Button } from "@/components/ui/button";

const steps = [
  {
    title: "Providers publish availability",
    body: "Set a start, an end, and it's live. Open slots appear in public search the moment they're created — closed slots disappear just as fast.",
  },
  {
    title: "Customers book in two clicks",
    body: "Browse real availability, pick a slot, add a note if you like. The booking is confirmed instantly and the slot locks for everyone else.",
  },
  {
    title: "The calendar stays honest",
    body: "One active booking per slot, enforced at the database level. Cancels reopen slots. Providers mark completions and no-shows from their dashboard.",
  },
];

const features = [
  {
    icon: ZapIcon,
    title: "Instant confirmations",
    body: "No approval queues. A booking is confirmed the moment it's made — the customer leaves with a real appointment.",
  },
  {
    icon: LockIcon,
    title: "Double-booking proof",
    body: "A unique constraint at the database layer guarantees one active booking per slot. Two customers clicking at once can't break it.",
  },
  {
    icon: CalendarClockIcon,
    title: "Timezone-aware by default",
    body: "Slot times carry the provider's timezone and render accordingly. Cross-country appointments, displayed honestly.",
  },
  {
    icon: SearchIcon,
    title: "Search that searches",
    body: "Filter public availability by provider and date range with server-side pagination — backed by Redis caching for speed.",
  },
  {
    icon: LockIcon,
    title: "Sessions that stay put",
    body: "Access and refresh tokens live in HttpOnly cookies managed by the server. Page JavaScript never sees a credential.",
  },
  {
    icon: BellRingIcon,
    title: "Notification-ready",
    body: "Booking events and delivery tracking are modeled in the platform core, ready to switch on when endpoints land.",
  },
];

export default function LandingPage() {
  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="bg-grid bg-fade-b absolute inset-0" aria-hidden="true" />
        <div className="relative mx-auto grid max-w-6xl items-center gap-14 px-4 pb-20 pt-16 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:gap-10 lg:pb-28 lg:pt-24">
          <div>
            <span className="inline-flex items-center gap-1.5 rounded-full border bg-card px-3 py-1 text-xs font-medium text-muted-foreground">
              <span className="size-1.5 rounded-full bg-brand" aria-hidden="true" />
              Real-time availability · instant confirmations
            </span>
            <h1 className="mt-5 text-4xl font-semibold tracking-tight text-balance sm:text-5xl lg:text-[3.4rem] lg:leading-[1.08]">
              Open a slot.
              <br />
              Watch it fill.
            </h1>
            <p className="mt-5 max-w-lg text-base leading-relaxed text-muted-foreground sm:text-lg">
              SlotForge turns your availability into a storefront. Publish open
              slots, and customers book confirmed appointments on the spot —
              no forms, no back-and-forth, no double-bookings.
            </p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <Button asChild size="lg">
                <Link href="/register">Create a free account</Link>
              </Button>
              <Button asChild size="lg" variant="outline">
                <a href="#how-it-works">See how it works</a>
              </Button>
            </div>
            <p className="mt-4 text-sm text-muted-foreground">
              Free for customers. Providers publish their first slot in minutes.
            </p>
          </div>
          <div className="px-2 sm:px-8 lg:px-0">
            <HeroVisual />
          </div>
        </div>
      </section>

      {/* How it works */}
      <section id="how-it-works" className="scroll-mt-16 border-t border-border/70 bg-card/40">
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 lg:py-24">
          <div className="max-w-xl">
            <h2 className="text-3xl font-semibold tracking-tight">
              Three moving parts. Zero coordination tax.
            </h2>
            <p className="mt-3 text-muted-foreground">
              The entire booking lifecycle, from a provider opening their
              calendar to a completed appointment.
            </p>
          </div>
          <ol className="mt-12 grid gap-10 lg:grid-cols-3 lg:gap-8">
            {steps.map((step, index) => (
              <li key={step.title} className="relative">
                <div className="flex items-center gap-3">
                  <span className="flex size-8 items-center justify-center rounded-full bg-primary font-mono text-sm font-medium text-primary-foreground">
                    {index + 1}
                  </span>
                  <div
                    className="hidden h-px flex-1 bg-gradient-to-r from-border to-transparent lg:block"
                    aria-hidden="true"
                  />
                </div>
                <h3 className="mt-4 font-semibold">{step.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                  {step.body}
                </p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="scroll-mt-16">
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 lg:py-24">
          <div className="max-w-xl">
            <h2 className="text-3xl font-semibold tracking-tight">
              Built like infrastructure, not a form builder.
            </h2>
            <p className="mt-3 text-muted-foreground">
              Transactional booking logic, cached reads and role-scoped
              access — the details that matter once real people depend on it.
            </p>
          </div>
          <div className="mt-12 grid gap-x-8 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((feature) => (
              <div key={feature.title}>
                <feature.icon className="size-5 text-brand" aria-hidden="true" />
                <h3 className="mt-3 font-semibold">{feature.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                  {feature.body}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Live availability preview */}
      <section id="availability" className="scroll-mt-16 border-t border-border/70 bg-card/40">
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 lg:py-24">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div className="max-w-xl">
              <h2 className="text-3xl font-semibold tracking-tight">
                On SlotForge right now
              </h2>
              <p className="mt-3 text-muted-foreground">
                This isn&apos;t a screenshot — it&apos;s the live public availability
                feed, straight from the booking engine.
              </p>
            </div>
            <Button asChild variant="ghost" size="sm">
              <Link href="/register">
                Publish your availability
                <CalendarRangeIcon aria-hidden="true" />
              </Link>
            </Button>
          </div>
          <div className="mt-10">
            <AvailabilityPreview />
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t border-border/70">
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
          <div className="relative overflow-hidden rounded-2xl bg-navy px-6 py-14 text-center sm:px-12">
            <div
              className="absolute inset-0 opacity-[0.14]"
              style={{
                backgroundImage:
                  "radial-gradient(circle at 20% 20%, rgb(241 124 105 / 0.55) 0%, transparent 45%), radial-gradient(circle at 80% 80%, rgb(241 124 105 / 0.55) 0%, transparent 45%)",
              }}
              aria-hidden="true"
            />
            <div className="relative">
              <h2 className="mx-auto max-w-xl text-3xl font-semibold tracking-tight text-cream text-balance">
                Stop asking “when works for you?”
              </h2>
              <p className="mx-auto mt-3 max-w-md text-cream/70">
                Put your availability where customers can act on it. The
                calendar does the rest.
              </p>
              <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
                <Button asChild size="lg" variant="brand">
                  <Link href="/register">
                    <UserPlusIcon aria-hidden="true" />
                    Get started free
                  </Link>
                </Button>
                <Button
                  asChild
                  size="lg"
                  variant="outline"
                  className="border-cream/25 bg-transparent text-cream hover:bg-cream/10 hover:text-cream"
                >
                  <Link href="/login">Sign in</Link>
                </Button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
