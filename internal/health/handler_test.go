package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newHealthRouter(pool *pgxpool.Pool, redis redisPinger) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router, pool, redis)

	return router
}

func TestHealthz(t *testing.T) {
	router := newHealthRouter(nil, nil)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var body map[string]string

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

// TestReadyzWithDatabase verifies readiness against the real PostgreSQL
// instance. Requires DATABASE_URL. The database may legitimately be in an
// unmigrated state (fresh dev database), so the test asserts that the
// reported checks are consistent with the status code rather than a
// specific outcome.
func TestReadyzWithDatabase(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	t.Cleanup(pool.Close)

	router := newHealthRouter(pool, nil)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	switch body.Status {
	case "ready":
		if response.Code != http.StatusOK {
			t.Fatalf("status ready but code %d", response.Code)
		}
		if body.Checks["database"] != "connected" ||
			body.Checks["schema"] != "ok" ||
			body.Checks["migrations"] != "ok" {
			t.Errorf("ready with unexpected checks: %v", body.Checks)
		}
	case "not_ready":
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status not_ready but code %d", response.Code)
		}
		failed := false
		for _, key := range []string{"database", "schema", "migrations"} {
			if body.Checks[key] != "ok" && body.Checks[key] != "connected" {
				failed = true
			}
		}
		if !failed {
			t.Errorf("not_ready but no failed check: %v", body.Checks)
		}
	default:
		t.Fatalf("unexpected status %q: %s", body.Status, response.Body.String())
	}

	// The response must never echo the connection string or credentials.
	if strings.Contains(response.Body.String(), "@") || strings.Contains(response.Body.String(), "postgres://") {
		t.Errorf("response leaks connection details: %s", response.Body.String())
	}
}

// TestReadyzWithUnavailableDatabase verifies the failure path without
// mocking: the pool points at a port nothing listens on, so the ping
// always fails and the endpoint must report 503.
func TestReadyzWithUnavailableDatabase(t *testing.T) {
	deadPool, err := pgxpool.New(context.Background(),
		"postgres://slotforge:unused@127.0.0.1:1/slotforge?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	t.Cleanup(deadPool.Close)

	router := newHealthRouter(deadPool, nil)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}

	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "not_ready" {
		t.Errorf("expected status not_ready, got %q", body.Status)
	}

	if body.Checks["database"] != "unavailable" {
		t.Errorf("expected database unavailable, got %q", body.Checks["database"])
	}
}

// TestReadyzRedisDegradedReported verifies a Redis outage is reported as
// degraded (fail-open), independent of the database state.
func TestReadyzRedisDegradedReported(t *testing.T) {
	router := newHealthRouter(nil, func(ctx context.Context) error {
		return errors.New("connection refused")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	// pool is nil so the ping fails — Redis alone must not change that,
	// but the redis check must report degraded, not ok.
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Checks["redis"] != "degraded" {
		t.Errorf("expected redis degraded, got %q", body.Checks["redis"])
	}
}

// TestReadyzRedisDisabled verifies the nil-pinger path.
func TestReadyzRedisDisabled(t *testing.T) {
	router := newHealthRouter(nil, nil)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Checks["redis"] != "disabled" {
		t.Errorf("expected redis disabled, got %q", body.Checks["redis"])
	}
}

// TestReadyzWithLiveRedis verifies the healthy Redis path when REDIS_ADDR
// points at a reachable instance.
func TestReadyzWithLiveRedis(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR is not set; skipping Redis integration test")
	}

	client := newTestRedisClient(addr)
	t.Cleanup(func() { _ = client.Close() })

	router := newHealthRouter(nil, func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Checks["redis"] != "ok" {
		t.Errorf("expected redis ok, got %q", body.Checks["redis"])
	}
}
