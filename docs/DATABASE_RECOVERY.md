# Database Recovery Runbook

Applies to the Render-hosted SlotForge PostgreSQL instance.

## Backup reality (read this first)

The database currently runs on the Render **free** plan, which provides **no
automated point-in-time recovery and no managed backups**. If the instance is
lost, everything since the last manual export is gone. The free instance also
expires ~30 days after creation (early December 2026 for the current one).

Mitigations, in order of preference:

1. **Upgrade the Render Postgres plan** to one that includes daily backups
   and PITR (paid plans). This is the only real fix.
2. **Scheduled manual dumps** (production runs **PostgreSQL 18.6**; the
   `postgres:18` Docker image ships an exact-match `pg_dump 18.6`, and the
   image is already pulled locally). Run from the repo root so the URL is
   read from `.env` and passed via the environment — it never appears in
   argv, the shell history, or this repo:

   ```sh
   # from the repo root
   export PGURL="$(grep '^SLOTFORGE_DB_URL=' .env | cut -d= -f2-)"
   mkdir -p ../slotforge-backups
   docker run --rm -e PGURL -v "$(cd ../slotforge-backups && pwd):/dumps" \
     postgres:18 \
     sh -c 'pg_dump "$PGURL" --no-owner --no-privileges -Fc -f "/dumps/slotforge-$(date +%F).dump"'
   unset PGURL
   ```

   Retain at least 7 days of dumps outside the laptop (e.g. private object
   storage). Never commit or paste the dump path list with credentials —
   dumps contain user password **hashes** and refresh-token hashes.

**Archive verification (do this before trusting any dump):**

```sh
docker run --rm -v "$(cd ../slotforge-backups && pwd):/dumps" postgres:18 \
  pg_restore --list "/dumps/slotforge-YYYY-MM-DD.dump" | head -20
```

Expect the table list (`users`, `availability_slots`, `bookings`,
`notifications`, `provider_unavailable_periods`, …) with no errors. A dump
that lists cleanly and was produced with `-Fc` is structurally sound.

**Test restore (isolated throwaway container — proves the dump restores,
touches nothing in production):**

```sh
# 1. throwaway server, isolated network namespace, test-only password
docker run --rm -d --name sf-restore-test -e POSTGRES_PASSWORD=test-only \
  postgres:18
docker exec sf-restore-test pg_isready -U postgres   # wait for "accepting connections"

# 2. restore into a scratch database inside the container
docker cp ../slotforge-backups/slotforge-YYYY-MM-DD.dump sf-restore-test:/tmp/
docker exec sf-restore-test createdb -U postgres slotforge_restore_test
docker exec sf-restore-test pg_restore -U postgres \
  -d slotforge_restore_test --no-owner --no-privileges /tmp/slotforge-YYYY-MM-DD.dump

# 3. verify content, then destroy everything
docker exec sf-restore-test psql -U postgres -d slotforge_restore_test -tc \
  "SELECT count(*) FROM schema_migrations;"                    # expect 6 (or the applied count at dump time)
docker exec sf-restore-test psql -U postgres -d slotforge_restore_test -tc \
  "SELECT count(*) FROM users;"                                # spot-check plausible, non-zero
docker rm -f sf-restore-test
```

A dump passes when the restore exits 0 and both spot-checks return sane
counts. Run a test restore for every new dump until the first two have
passed, then monthly.

## Recovery procedure (from a dump)

1. Provision a fresh PostgreSQL instance (Render or other; it must run
   PostgreSQL **18.x** to match the dump) and copy its **external**
   connection URL into `.env` as `DATABASE_URL`.
2. Restore (Docker form, same convention as the backup — URL via
   environment, never argv):

   ```sh
   export PGURL="$(grep '^DATABASE_URL=' .env | cut -d= -f2-)"
   docker run --rm -e PGURL -v "$(cd ../slotforge-backups && pwd):/dumps" \
     postgres:18 \
     sh -c 'pg_restore "$PGURL" --no-owner --no-privileges --clean --if-exists "/dumps/slotforge-YYYY-MM-DD.dump"'
   unset PGURL
   ```

3. Verify: `readyz` on a locally started API reports `migrations: ok`
   (it checks required tables and the applied migration list), and
   `schema_migrations` holds the applied count (6 after migration 006).
4. Update `DATABASE_URL` on the Render API service (dashboard, environment)
   and restart. The frontend needs no change (it talks to the API only).

## Migration rollback strategy

Migrations are forward-only by design; there are no down-migrations.

- Every migration file runs in a single transaction; a failed migration
  leaves no partial changes.
- To undo a bad forward migration, write a **new** forward migration
  (`00N_undo_...`) that reverses the change, respecting foreign keys
  (drop children before parents: deliveries → notifications → refresh
  tokens → bookings → slots → profiles → users).
- Restore-from-backup is the fallback for data-level corruption; see above.
- Apply migrations with `node scripts/apply-migrations.mjs "<url>"` from
  `frontend/` (the runner is idempotent: it records applied filenames in
  `schema_migrations` and skips them).

## Connection pool and query timeouts (set in Phase 18)

| Setting | Value | Where |
|---|---|---|
| Pool max connections | 20 | `internal/database/postgres.go` |
| Pool min connections | 2 | same |
| Connection lifetime | 1 h | same |
| Idle connection lifetime | 30 min | same |
| `statement_timeout` | 15 s | same |
| `idle_in_transaction_session_timeout` | 30 s | same |

The timeouts bound runaway queries and leaked transactions; booking row locks
are held only for the duration of short transactions, so 15 s is safe headroom.
If a query legitimately needs longer, raise `statement_timeout` consciously —
do not remove it.

## Diagnosing long-running queries

```sql
SELECT pid, state, now() - query_start AS running_for, left(query, 80)
FROM pg_stat_activity
WHERE datname = current_database() AND state <> 'idle'
ORDER BY running_for DESC;
```

Kill a stuck query with `SELECT pg_cancel_backend(<pid>);` (or
`pg_terminate_backend` if it ignores cancellation). Locks waiters can be read
from `pg_locks` joined on `pg_stat_activity`.

## Known limitations

- Free plan: no backups, no PITR, instance expiry (~early Dec 2026).
- Restore-from-dump recreates schema and data but Render's **Redis** cache is
  not part of any backup; it is disposable by design (fail-open) and can be
  left empty after recovery.
- The API caches nothing critical; a restored database needs no cache warm-up.
