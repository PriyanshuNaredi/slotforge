# SlotForge Frontend

A polished Next.js (App Router) frontend for the SlotForge booking platform —
landing site, customer dashboard, and provider workspace, all wired to the
existing Go API with **zero backend changes**.

## Stack

- Next.js 16 (App Router, Turbopack) + React 19, TypeScript strict
- Tailwind CSS v4 with a custom SlotForge design system (`src/app/globals.css`)
- shadcn/ui-style components vendored under `src/components/ui` (Radix primitives)
- react-hook-form + zod, lucide-react, motion, sonner
- Vitest for journey-critical logic

## Getting started

```bash
npm install
cp .env.example .env.local   # points the proxy at http://127.0.0.1:8080
npm run dev                  # http://localhost:3000
```

Requires the Go API to be running (repo root: `go run ./cmd/api` with Postgres
and Redis up).

## Architecture

### Auth: HttpOnly cookies + proxy (no backend changes)

The Go API is Bearer-token based and has no CORS middleware, so:

1. **Everything is same-origin.** The browser only ever talks to
   `localhost:3000/api/...`. `next.config.ts` rewrites `/api/v1/*` to the Go API
   (`API_PROXY_ORIGIN`, server-side only). No tokens in `NEXT_PUBLIC_*`.
2. **Tokens live in HttpOnly cookies.** `src/app/api/auth/*` route handlers
   broker login/register/refresh against the backend and set `sf_access`
   (15 min) and `sf_refresh` (7 days, rotated by the backend on every refresh).
3. **`src/proxy.ts` injects credentials.** For `/api/v1/*` requests it attaches
   `Authorization: Bearer <cookie>` and lets the rewrite pass through. It also
   gates `/dashboard` (customer) and `/provider` (provider) by the role decoded
   from the cookie — for routing UX only; the backend enforces real authz on
   every call.
4. **401 → single-flight refresh → retry.** `src/lib/api/client.ts` performs
   one shared refresh via `/api/auth/refresh`, retries the original request
   once, and surfaces typed `ApiError`s otherwise.

### API layer

- `src/lib/api/types.ts` — DTOs mirroring the Go models exactly
- `src/lib/api/client.ts` — fetch wrapper, `ApiError`, refresh flow
- `src/lib/api/endpoints.ts` — one typed function per backend endpoint;
  notifications (list, unread count, mark read, mark all) are fully wired

### Data fetching

`src/hooks/use-async-data.ts` is the canonical client fetch hook (one request
per key, stale reads while reloading, typed errors). Bookings are hydrated
with slot details via `src/hooks/use-slot-details.ts`; the public slot endpoint
404s for closed/past slots, which the UI degrades on gracefully.

### Timezones

Slots carry the provider's IANA timezone. `src/lib/format.ts` renders slot
times in the provider's timezone (with abbreviation), and converts the
provider's local input to UTC ISO when creating slots — DST-safe round trip,
covered by tests.

## Provider accounts

`POST /auth/register` always creates customers (hardcoded in the backend).
Seed a provider the same way the platform's own integration tests do:

```bash
npm run seed -- --email provider@example.com --password "at-least-12-chars" --name "Dr. Example" --slots 3
```

## Quality gates

```bash
npm run lint     # eslint (incl. react-compiler rules)
npx tsc --noEmit # strict types
npm run test     # vitest
npm run build    # production build
```

## Known limitations (honest scope)

- Provider profile editing: the backend only supports create-once profiles.
- Logout clears browser cookies but does NOT revoke the backend refresh
  token (no logout endpoint exists) — the token stays valid server-side until
  its natural expiry.
- Refresh-token rotation is single-use (no grace window): two tabs refreshing
  simultaneously can log one out — backend design, not fixable client-side.
- Cookie TTLs assume the backend defaults (15 min access / 7-day refresh);
  changing those backend env vars requires matching updates in
  `src/lib/server/auth-cookies.ts`.
- No E2E browser tests yet; journey logic (validation, status transitions,
  error mapping, refresh flow, timezone math, notifications) is unit-tested.
