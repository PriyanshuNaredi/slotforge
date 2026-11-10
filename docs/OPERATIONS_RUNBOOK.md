# Operations Runbook — SlotForge

Production: API `https://slotforge-api.onrender.com`, frontend
`https://slotforge-frontend.onrender.com`. Never paste secrets into this
repo; the git-ignored `.env` holds the external DB URL.

## Health checks

- `GET /healthz` — liveness; always 200 when the process is up.
- `GET /readyz` — readiness; verifies PostgreSQL, required tables, applied
  migrations, and reports Redis state (`ok`/`degraded`/`disabled`). Redis
  being down does **not** fail readiness (fail-open design). 503 = do not
  route traffic; check the specific `checks` object.

## Deploy procedure

1. Land changes on `main` (commits require explicit user approval).
2. Push → Render auto-deploys backend (Docker) and frontend (Next.js).
3. Watch `readyz` until 200 (or apply a pending migration first, see below).
4. Smoke: login against the prod API; load the marketing page.

## Migration procedure (prod — requires explicit user approval)

```sh
node frontend/scripts/apply-migrations.mjs "<render-external-db-url>"
```

- One transaction per file; safe to re-run.
- Order matters for combined deploy+migration: apply the migration **before**
  deploying code that requires the new schema. Current pending: `006_provider_unavailable_periods.sql`.
- `/readyz` reports `migrations: not_applied` when the recorded set is behind
  the code's expectation — that is the signal to run a migration.

## Incident response (first 10 minutes)

1. `readyz` — classify: database / schema / migrations / redis.
2. Render dashboard → API service → Logs. Structured JSON lines carry
   `request_id`; correlate a failing request from the frontend error page's
   `X-Request-ID` response header.
3. Database down → check the Render Postgres instance status; see
   DATABASE_RECOVERY.md if the instance is lost.
4. Redis down → nothing to do: cache fails open; fix when convenient.
   `readyz` will show `redis: degraded`.
5. Email delivery stalled → logs for `email_worker.*` events; deliveries
   retry with exponential backoff (30 s → 1 h cap, 5 attempts) then park in
   `failed`. `RecoverStaleClaims` requeues jobs from crashed workers every
   minute.
6. Login lockouts (429) → per-account throttle: 10 failed logins / 5 min.
   Successful login resets. No operator unlock action exists (wait out the
   window).

## Rollback procedure

1. Render dashboard → the service's previous deploy → "Restore". Rollbacks
   are fast and safe for stateless API/frontend containers.
2. If the bad deploy shipped a **migration**, do not roll back the code
   alone: forward-migrations are additive by convention (`006` only adds a
   table + index), so an older code version keeps working against the newer
   schema. Never hand-edit `schema_migrations`.
3. Data-level rollback = restore from dump (DATABASE_RECOVERY.md).

## Monitoring (what exists today)

- Render service dashboards (CPU, memory, restarts).
- `readyz` as an external uptime check target.
- Structured JSON logs (stdout) — grep-able events:
  `http_request`, `auth_login_succeeded`/`auth_login_failed`,
  `booking_created|cancelled|rescheduled`, `email_worker.delivered`,
  `email_worker.send_failed`, `email_worker.stale_claims_recovered`,
  `panic_recovered`. No passwords, tokens, or emails are logged.

## Known operational limits

- Free-tier Postgres: no managed backups, expires ~early Dec 2026 → upgrade
  or export (DATABASE_RECOVERY.md).
- Single API instance; deploys cause brief 503s during restart.
- Render free services sleep after idle; first request may be slow.
