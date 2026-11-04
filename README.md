# SlotForge

Appointment-booking platform: a Go/Gin API (PostgreSQL, Redis, JWT) with a
Next.js frontend.

## Prerequisites

- Go 1.26+
- Node.js 20+ and npm
- Docker (for PostgreSQL and Redis)

## Backend

1. Copy the example env and fill in real values (never commit `.env`):

   ```bash
   cp .env.example .env
   ```

   `POSTGRES_PASSWORD` (used by Docker Compose) must match the password in
   `DATABASE_URL`.

2. Start PostgreSQL and Redis:

   ```bash
   docker compose up -d
   ```

   Note: Postgres reads `POSTGRES_PASSWORD` only when its volume is first
   initialized. To change it later, remove the volume (`docker compose down -v`)
   — this deletes local data.

3. Run migrations and start the API:

   ```bash
   go run ./cmd/api
   ```

   The API listens on `:8080` (`/healthz`, `/readyz` for liveness).

## Frontend

```bash
cd frontend
npm install
cp .env.example .env.local   # points the API proxy at http://127.0.0.1:8080
npm run dev                  # http://localhost:3000
```

See [frontend/README.md](frontend/README.md) for architecture, auth design,
provider seeding, and quality gates.

## Tests

```bash
go test ./... -count=1      # backend (integration tests use DATABASE_URL)
go vet ./...
cd frontend
npm run test                # vitest
npx tsc --noEmit && npx eslint src && npm run build
```
