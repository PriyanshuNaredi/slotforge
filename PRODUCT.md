# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Customers** (primary): people looking for a service provider — finding availability, booking, rescheduling, and managing their appointments. Arrive via the public site, often first-time.
- **Providers** (secondary but equally served daily): independent service businesses (e.g. a dermatology clinic) publishing their availability and managing incoming bookings. Live in the provider workspace for scheduled work, not casual browsing.

## Product Purpose

SlotForge is an appointment-booking platform built on real-time availability. Providers publish open time slots; customers discover providers, see genuinely open slots, and book instantly. Success = a booked appointment with zero back-and-forth, and calendars that stay honest (no double-booking, no stale slots).

## Positioning

Real-time slot publishing: what the customer sees is the provider's live, truth-enforced availability — not a request queue. The mechanism a competitor could not copy without the same backend guarantees: slot-level locking, instant confirmation, and status transitions that keep both calendars honest.

## Operating Context

- Two authenticated experiences reached from a public marketing site: `/dashboard/*` (customer) and `/provider/*` (provider workspace), each with a persistent sidebar nav.
- Provider accounts exist only via a seed script (`npm run seed` in `frontend/`); self-registration always creates customers.
- Seeded demo accounts for live testing: `provider-demo@slotforge.test` / `demo-provider-pass-1`, `customer-demo@slotforge.test` / `demo-customer-pass-1`.
- Deployed (in progress) on Render; local dev runs the Go API plus `next dev`.

## Capabilities and Constraints

- Next.js 16 (App Router, Turbopack), React 19, TypeScript, Tailwind v4, FullCalendar v7, Radix primitives, motion, sonner, zod + react-hook-form.
- Routes, API contracts, cookie auth, notifications, rescheduling, and availability behavior are fixed — this is a visual redesign, not a feature rewrite. No Go backend or migration changes.
- No fake data in authenticated dashboards; no removal of working functionality; no new dependencies without need.
- FullCalendar functionality and existing booking/rescheduling interactions must keep working.

## Brand Commitments

- Name: **SlotForge**. Voice: confident, modern, trustworthy, precise, approachable — professional enough for real businesses, never playful/childish.
- Color identity: **navy and coral** as the brand; **cream returns as the supporting page surface** (Phase 14D decision — the all-paper timetable look read as too plain and static). Keep strong contrast; avoid an all-pale application.
- Direction (Phase 14D, user-pinned): **modern, warm, polished scheduling SaaS with a lively product preview** — layered hero card, floating confirmation/calendar elements, subtle CSS motion. The railway-timetable concept is retired as the global direction; its legibility wins (status stamps, calendar legend, tabular times, unified shell) are kept.
- Phase 14B/14C functional and accessibility wins are binding: fixed booking-success flow, correct dialog semantics, per-type notification icons, calendar event contrast + legend, WCAG AA, visible focus, reduced-motion support.

## Evidence on Hand

- Current-state screenshots of dashboards, calendars, booking flows: `docs/screenshots/`.
- No testimonials, customer logos, benchmarks, or press. None may be fabricated.

## Product Principles

1. **The calendar is the truth.** Every screen reflects real availability and real bookings; the interface never implies state the backend doesn't confirm.
2. **Seconds to booked.** From "I need an appointment" to confirmed slot in as few decisions as possible.
3. **Two sides, one language.** A booking means the same thing (same status vocabulary, same time precision) to provider and customer.
4. **Precision reads as competence.** Exact times, explicit timezones, unambiguous statuses — this is how the product earns trust.

## Accessibility & Inclusion

WCAG AA contrast, visible focus states, reduced-motion support, and keyboard-operable flows are required by the Phase 14B brief.
