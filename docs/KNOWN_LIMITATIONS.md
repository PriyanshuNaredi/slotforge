# Known Limitations — SlotForge

Honest ledger of what SlotForge does **not** do, verified as of 2026-11-07.

## Operational

1. **Free-tier database, no backups, expiry.** The Render Postgres instance
   is on the free plan: no managed backups or PITR, and the instance expires
   ~early December 2026. Manual dumps are the only recovery path until the
   plan is upgraded (docs/DATABASE_RECOVERY.md).
2. **Single API instance.** Deploys and crashes interrupt in-flight
   requests; free services also sleep when idle (slow first request).
3. **No metrics endpoint.** Observability = structured JSON logs and
   `readyz`; there is no Prometheus/OpenTelemetry integration. Delivery
   counts are log-derivable, not exposed programmatically.
4. **Prod schema is one migration behind the code until 006 is applied**;
   `readyz` will (correctly) report `migrations: not_applied` after the new
   backend deploys without it.

## Product behavior

5. **Email delivery is at-least-once.** A crash between "provider accepted
   the email" and "MarkSent" requeues the job → the recipient may get a
   duplicate email. Without `RESEND_API_KEY`/`EMAIL_FROM`, notifications are
   in-app only (worker disabled).
6. **No provider-profile update endpoint** (create + read only); edits
   require SQL or a future endpoint.
7. **No admin role UI/API surface** beyond the DB-level role support.
8. **No API creates provider accounts** — provider users must be seeded via
   SQL (documented pattern; demo/e2e accounts only).
9. **Slots are single-appointment** (no capacity/group bookings), and
   booking into a provider-blocked window is refused rather than queued.
10. **Blocking over active bookings is refused** (the provider must cancel
    first); the system never silently cancels confirmed bookings.
11. **Recurring availability horizon** is capped at 90 days / 500 slots per
    request; overnight windows (end ≤ start) are rejected.
12. **Login lockout DoS**: per-account throttling means an attacker can
    intentionally lock a victim out for rolling 5-minute windows.
13. **No MFA.**
14. **Notifications are pull-based**: 60 s badge polling plus event-driven
    refresh; no WebSocket push. Reads can lag up to a minute for
    server-side events.

## Technical debt

15. `npm audit` shows 8 moderate advisories, all inside the dev-only eslint
    toolchain (minimatch). Not runtime-reachable; fixing requires breaking
    lint dependency upgrades.
16. Frontend redesign work (Phase 14B) and the Phases 18–19 backend changes
    are uncommitted/unpushed as of this writing — commit plan awaits user
    approval.
17. The local dev database had an empty `schema_migrations` table (Phase 16
    leftover); fixed locally by marking 001–005 applied, then applying 006.
18. Load-test numbers (docs/PRODUCTION_READINESS.md) are from a local dev
    machine, not production-sized hardware; they demonstrate correctness
    under concurrency, not production capacity.
