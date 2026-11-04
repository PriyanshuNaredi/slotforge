# Deploying SlotForge to Render

The repo ships a [Render Blueprint](render.yaml) that provisions everything:
Go API (Docker), Next.js frontend, PostgreSQL, and Redis. Zero backend code
changes are required — the frontend proxies all API traffic through its own
origin (`API_PROXY_ORIGIN`), so no CORS setup is needed.

## One-time setup

1. Go to <https://dashboard.render.com> and sign in (GitHub account works).
2. **New + → Blueprint**, authorize Render for GitHub if asked, select the
   `PriyanshuNaredi/slotforge` repo, branch `main`.
3. Render reads `render.yaml` and shows the services to create. It will
   prompt for two values:
   - `API_PROXY_ORIGIN` — the public URL of the API service. Render names it
     `slotforge-api`, so enter `https://slotforge-api.onrender.com`
     (adjust if Render appends a suffix — copy the exact URL shown on the
     API service page).
   - `RESEND_API_KEY` — leave **empty** to disable email delivery, or paste
     a Resend key to enable it.
4. Click **Apply**. First deploys take a few minutes.

## Apply database migrations

Render creates an empty database; the schema must be applied once:

```bash
cd frontend
node scripts/apply-migrations.mjs "<Render-Postgres-External-Database-URL>"
```

The URL is in the Render dashboard: `slotforge-db` → **Connections →
External Database URL**. The script is idempotent — safe to rerun.

## Post-deploy checklist

- `https://<api-url>/healthz` → `{"status":"ok"}`
- `https://<api-url>/readyz` → 200 once the database is reachable
- `https://<frontend-url>` → landing page, and the live availability
  section shows data
- Register a customer, seed a provider (see frontend README), book a slot

## Free-tier caveats

- Free web services sleep after ~15 min idle; the first request after a nap
  takes ~30–60 s.
- Free PostgreSQL expires after **30 days** — upgrade the instance or export
  data before then.
- Free Redis is 25 MB without persistence (the API treats Redis as an
  optional cache and fails open without it).

## Environment variables (production)

| Service | Variable | Source |
|---|---|---|
| API | `DATABASE_URL` | Render Postgres (auto-wired) |
| API | `REDIS_ADDR` | Render Redis (auto-wired) |
| API | `JWT_SECRET` | Render-generated |
| API | `APP_ENV` | `production` |
| API | `RESEND_API_KEY` | Optional, manual |
| Frontend | `API_PROXY_ORIGIN` | Public URL of the API service |
| Frontend | `NODE_VERSION` | `22` |

Secrets live only in Render's environment settings — never in the repo.
