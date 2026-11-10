# Production Readiness Report — SlotForge

Date: 2026-11-07 · Scope: Phases 17B–20 release-readiness work.

## Verified capability summary

| Area | State | Evidence |
|---|---|---|
| Auth (register/login/refresh/logout/revocation) | Verified in production | 60/60 E2E checks vs `slotforge-api.onrender.com` |
| Booking lifecycle + concurrency | Verified in production + load-tested locally | 50/50 same-slot races resolved 1×201/1×409; partial unique index |
| Notifications (list/unread/read/read-all/ownership) | Verified in production | E2E checks N1–N8 |
| Authorization boundaries | Verified in production | E2E checks A1–A15 (404-not-403 for foreign customers, 403 for role mismatch) |
| Frontend↔API proxy | Verified in production | `GET /api/v1/providers` via frontend → 200 |
| Readiness probing | Phase 18 code, unit+integration tested | `internal/health` (DB, schema, migrations, Redis fail-open) |
| Structured logging / request IDs | Phase 18 code, unit tested | `internal/logging`; JSON lines with `request_id` |
| Login brute-force throttle | Phase 18 code, unit tested | 10 failed attempts / 5 min per account → 429 |
| Security headers | Phase 18 code, tested | nosniff, DENY, Referrer-Policy, HSTS |
| DB statement/idle-tx timeouts | Phase 18 code, integration tested | 15 s / 30 s in `internal/database/postgres.go` |
| Email delivery worker | Phase 18 review + pre-existing failure-path tests | at-least-once semantics, retry/backoff, stale-claim recovery |
| Recurring availability | Phase 19 code, unit+integration tested | DST tests across 2026 spring-forward/fall-back |
| Unavailable periods | Phase 19 code, integration tested | blocks bookings/reschedules; never cancels silently |
| Booking pagination | Phase 19 (totals pre-existing; stable order added) | limit/offset/total on both list endpoints |
| Historical booking detail | Phase 19 regression test (join existed since Phase 15) | past+closed slots resolve |
| Browser E2E | Phase 20, local isolated stack | Playwright 14/14 (12 journeys + a11y) |
| Accessibility | Phase 20 | axe: no serious/critical violations; keyboard/dialog/focus checks pass |

## Deployment status (not yet executed — needs approvals)

The following are **implemented and tested locally but not deployed to
production**; each requires the explicit approvals listed in
OPERATIONS_RUNBOOK.md:

1. Backend changes (Phases 18–19) — commit → push → Render deploy.
2. Migration `006_provider_unavailable_periods.sql` — apply to prod DB.
3. Frontend change (unread-count polling) — rides along with the next
   frontend deploy.

`/readyz` deliberately requires migration 006 to be applied before reporting
ready, so: apply migration first, then deploy — or deploy and accept a short
`not_ready` window while the migration runs.

## Environment variables (API)

| Var | Required | Notes |
|---|---|---|
| `DATABASE_URL` | yes | Render internal URL in prod |
| `JWT_SECRET` | yes | ≥32 chars |
| `JWT_ACCESS_TTL` / `REFRESH_TOKEN_TTL` | no | 15 m / 168 h defaults |
| `REDIS_ADDR` | no | host:port or redis:// URL; fail-open |
| `APP_PORT` / `APP_ENV` | no | Render injects PORT/ENV |
| `EMAIL_WORKER_ENABLED` | no | default true |
| `RESEND_API_KEY` / `EMAIL_FROM` | for email | worker disabled without them |
| `EMAIL_WORKER_BATCH_SIZE` / `_POLL_INTERVAL` / `_RECOVERY_INTERVAL` / `_STALE_AFTER` | no | sane defaults; batch×send-timeout must stay under stale-after |

Frontend: `API_PROXY_ORIGIN` must point at the API (rewrites bake at build
time — a rebuild is required after changing it).

## Migration procedure

```sh
node frontend/scripts/apply-migrations.mjs "<external-db-url>"
```

Idempotent, one transaction per file, recorded in `schema_migrations`.
Forward-only; rollback strategy and recovery in DATABASE_RECOVERY.md.

## Remaining limitations

See KNOWN_LIMITATIONS.md. Release decision inputs: the app is functionally
complete and verified; the *deploy* of Phases 18–19 work and migration 006
is outstanding, and the free-tier database (no backups, expires ~early Dec
2026) is the single largest operational risk.
