package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newHealthRouter(pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router, pool)

	return router
}

func TestHealthz(t *testing.T) {
	router := newHealthRouter(nil)

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

// TestReadyzWithDatabase verifies the readiness endpoint against the real
// PostgreSQL instance. Requires DATABASE_URL.
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

	router := newHealthRouter(pool)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var body map[string]string

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ready" {
		t.Errorf("expected status ready, got %q", body["status"])
	}

	if body["database"] != "connected" {
		t.Errorf("expected database connected, got %q", body["database"])
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

	router := newHealthRouter(deadPool)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}

	var body map[string]string

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "not_ready" {
		t.Errorf("expected status not_ready, got %q", body["status"])
	}

	if body["error"] == "" {
		t.Error("expected an error field on failure")
	}
}
