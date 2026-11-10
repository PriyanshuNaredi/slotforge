import Link from "next/link";
import {
  BellRingIcon,
  CalendarClockIcon,
  CalendarRangeIcon,
  CheckCircle2Icon,
  LockIcon,
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

const featureGroups = [
  {
    audience: "For customers",
    items: [
      {
        icon: ZapIcon,
        title: "Booked means booked",
        body: "Your confirmation is instant, and the slot locks for everyone else the moment it's yours.",
      },
      {
        icon: CalendarClockIcon,
        title: "Every time, in your time",
        body: "Slots carry the provider's timezone and render in yours. What you see on screen is what you attend.",
      },
      {
        icon: CheckCircle2Icon,
        title: "Your appointments, one list",
        body: "Book, reschedule, or cancel from a single place — with the full history kept and legible.",
      },
    ],
  },
  {
    audience: "For providers",
    items: [
      {
        icon: CalendarRangeIcon,
        title: "Open a slot, watch it fill",
        body: "Publish availability in seconds; customers act on it without a single phone call.",
      },
      {
        icon: LockIcon,
        title: "Every ending, on record",
        body: "Mark completions and no-shows as they happen. Your calendar reflects what actually occurred.",
      },
      {
        icon: BellRingIcon,
        title: "Availability that manages itself",
        body: "A cancelled booking reopens its slot automatically. The published calendar is always the truth.",
      },
    ],
  },
];

export default function LandingPage() {
  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="relative mx-auto grid max-w-6xl items-center gap-14 px-4 pb-20 pt-16 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:gap-10 lg:pb-28 lg:pt-24">
          <div>
            <span
              className="animate-fade-up inline-flex items-center gap-1.5 rounded-full border bg-card px-3 py-1 text-xs font-medium text-ink-soft"
              style={{ animationDelay: "0ms" }}
            >
              <span className="size-1.5 rounded-full bg-brand" aria-hidden="true" />
              Real-time availability · instant confirmations
            </span>
            <h1
              className="animate-fade-up mt-5 font-display text-4xl font-semibold tracking-tight text-balance sm:text-5xl lg:text-[3.4rem] lg:leading-[1.08]"
              style={{ animationDelay: "80ms" }}
            >
              Open a slot.
              <br />
              Watch it fill.
            </h1>
            <p
              className="animate-fade-up mt-5 max-w-lg text-base leading-relaxed text-ink-soft sm:text-lg"
              style={{ animationDelay: "160ms" }}
            >
              SlotForge turns your availability into a storefront. Publish open
              slots, and customers book confirmed appointments on the spot —
              no forms, no back-and-forth, no double-bookings.
            </p>
            <div
              className="animate-fade-up mt-8 flex flex-wrap items-center gap-3"
              style={{ animationDelay: "240ms" }}
            >
              <Button asChild size="lg">
                <Link href="/register">Create a free account</Link>
              </Button>
              <Button asChild size="lg" variant="outline">
                <a href="#how-it-works">See how it works</a>
              </Button>
            </div>
            <p
              className="animate-fade-up mt-4 border-t border-border/70 pt-4 text-sm text-ink-soft"
              style={{ animationDelay: "320ms" }}
            >
              Free for customers.{" "}
              <Link
                href="/register"
                className="inline-flex items-center gap-1 font-medium text-brand-strong underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-coral"
              >
                Providers: publish your availability
              </Link>
            </p>
          </div>
          <div
            className="animate-fade-up px-2 sm:px-8 lg:px-0"
            style={{ animationDelay: "200ms" }}
          >
            <HeroVisual />
          </div>
        </div>
      </section>

      {/* How it works */}
      <section id="how-it-works" className="scroll-mt-16 border-t border-border/70 bg-card/40">
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 lg:py-24">
          <div className="max-w-xl">
            <h2 className="font-display text-3xl font-semibold tracking-tight text-balance">
              Three moving parts. Zero coordination tax.
            </h2>
            <p className="mt-3 text-ink-soft">
              The entire booking lifecycle, from a provider opening their
              calendar to a completed appointment.
            </p>
          </div>
          <ol className="mt-12 grid gap-10 lg:grid-cols-3 lg:gap-8">
            {steps.map((step, index) => (
              <li
                key={step.title}
                style={{ animationDelay: `${index * 120}ms` }}
                className="animate-fade-up relative"
              >
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
                <p className="mt-2 text-sm leading-relaxed text-ink-soft">
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
            <h2 className="font-display text-3xl font-semibold tracking-tight text-balance">
              Built for the moment of booking.
            </h2>
            <p className="mt-3 text-ink-soft">
              Transactional booking logic underneath; a calendar both sides can
              trust on the surface.
            </p>
          </div>
          <div className="mt-12 grid gap-x-8 gap-y-12 lg:grid-cols-2">
            {featureGroups.map((group) => (
              <div key={group.audience}>
                <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-brand-strong">
                  <span className="h-px w-6 bg-coral" aria-hidden="true" />
                  {group.audience}
                </h3>
                <div className="mt-5 grid gap-x-6 gap-y-8 sm:grid-cols-3">
                  {group.items.map((item) => (
                    <div key={item.title}>
                      <item.icon className="size-5 text-brand" aria-hidden="true" />
                      <h4 className="mt-3 text-sm font-semibold">{item.title}</h4>
                      <p className="mt-1.5 text-sm leading-relaxed text-ink-soft">
                        {item.body}
                      </p>
                    </div>
                  ))}
                </div>
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
              <h2 className="font-display text-3xl font-semibold tracking-tight text-balance">
                On SlotForge right now
              </h2>
              <p className="mt-3 text-ink-soft">
                This isn&apos;t a screenshot — it&apos;s the live public
                availability feed, straight from the booking engine.
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
            <AvailabilityPreview limit={6} />
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
              <h2 className="mx-auto max-w-xl font-display text-3xl font-semibold tracking-tight text-cream text-balance">
                Stop asking &ldquo;when works for you?&rdquo;
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
