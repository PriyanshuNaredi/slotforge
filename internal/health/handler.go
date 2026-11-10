package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// requiredTables must exist in schema public for the API to serve requests.
var requiredTables = []string{
	"users",
	"refresh_tokens",
	"provider_profiles",
	"availability_slots",
	"bookings",
	"notifications",
	"notification_deliveries",
	"provider_unavailable_periods",
	"schema_migrations",
}

// requiredMigrations are the migration filenames recorded in
// schema_migrations. Update this list whenever a new migration is added.
var requiredMigrations = []string{
	"001_initial_schema.sql",
	"002_authentication.sql",
	"003_add_availability_slots_updated_at.sql",
	"004_notifications.sql",
	"005_notification_deliveries.sql",
	"006_provider_unavailable_periods.sql",
}

// redisPinger abstracts the Redis client so readiness can be tested without
// a live Redis. Redis is optional by design (fail-open), so a Redis failure
// degrades the report but never fails readiness.
type redisPinger func(ctx context.Context) error

func RegisterRoutes(router *gin.Engine, pool *pgxpool.Pool, redis redisPinger) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := gin.H{}
		ready := true

		if pool == nil {
			checks["database"] = "unavailable"
			ready = false
		} else if err := pool.Ping(ctx); err != nil {
			checks["database"] = "unavailable"
			ready = false
		} else {
			checks["database"] = "connected"

			var tables int
			if err := pool.QueryRow(ctx,
				`SELECT count(*)::int FROM information_schema.tables
				 WHERE table_schema = 'public' AND table_name = ANY($1)`,
				requiredTables,
			).Scan(&tables); err != nil || tables < len(requiredTables) {
				checks["schema"] = "incomplete"
				ready = false
			} else {
				checks["schema"] = "ok"
			}

			var migrations int
			if err := pool.QueryRow(ctx,
				`SELECT count(*)::int FROM public.schema_migrations WHERE name = ANY($1)`,
				requiredMigrations,
			).Scan(&migrations); err != nil || migrations < len(requiredMigrations) {
				checks["migrations"] = "not_applied"
				ready = false
			} else {
				checks["migrations"] = "ok"
			}
		}

		switch {
		case redis == nil:
			checks["redis"] = "disabled"
		default:
			pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)
			defer pingCancel()
			if err := redis(pingCtx); err != nil {
				// Fail-open: cache operations degrade, API still serves.
				checks["redis"] = "degraded"
			} else {
				checks["redis"] = "ok"
			}
		}

		status := "ready"
		code := http.StatusOK
		if !ready {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, gin.H{
			"status": status,
			"checks": checks,
		})
	})
}
