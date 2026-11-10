package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestEngine(logger *slog.Logger) (*gin.Engine, *bytes.Buffer) {
	gin.SetMode(gin.TestMode)
	buf := &bytes.Buffer{}
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(buf, nil))
	}
	router := gin.New()
	router.Use(RequestID(), Access(logger), Recovery(logger))
	return router, buf
}

func decodeLast(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var out map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &out); err != nil {
		t.Fatalf("decode log line: %v (%q)", err, buf.String())
	}
	return out
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	router, _ := newTestEngine(nil)
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, RequestIDFrom(c.Request.Context()))
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if res.Body.String() == "" {
		t.Fatal("expected a generated request ID in context")
	}
	if got := res.Header().Get("X-Request-ID"); got != res.Body.String() {
		t.Fatalf("header %q != context %q", got, res.Body.String())
	}
}

func TestRequestIDHonoursInboundHeader(t *testing.T) {
	router, _ := newTestEngine(nil)
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, RequestIDFrom(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "incoming-id")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Body.String() != "incoming-id" {
		t.Fatalf("expected inbound request ID, got %q", res.Body.String())
	}
}

func TestAccessLogsRequestFields(t *testing.T) {
	router, buf := newTestEngine(nil)
	router.GET("/ping", func(c *gin.Context) {
		c.Set("userID", "user-123")
		c.Status(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	entry := decodeLast(t, buf)
	if entry["msg"] != "http_request" {
		t.Fatalf("expected http_request, got %v", entry["msg"])
	}
	if entry["method"] != "GET" || entry["path"] != "/ping" || entry["status"] != float64(200) {
		t.Fatalf("missing request fields: %v", entry)
	}
	if entry["user_id"] != "user-123" {
		t.Fatalf("expected user_id, got %v", entry["user_id"])
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatal("expected duration_ms")
	}
}

func TestAccessOmitsUserIDWhenAbsent(t *testing.T) {
	router, buf := newTestEngine(nil)
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	entry := decodeLast(t, buf)
	if _, ok := entry["user_id"]; ok {
		t.Fatalf("user_id must be absent for anonymous requests: %v", entry)
	}
}

func TestSecurityHeadersSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/ping", nil))

	for _, h := range []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Referrer-Policy",
		"Strict-Transport-Security",
	} {
		if res.Header().Get(h) == "" {
			t.Errorf("missing security header %s", h)
		}
	}
}

func TestRecoveryReturns500WithoutLeakingPanic(t *testing.T) {
	router, buf := newTestEngine(nil)
	router.GET("/boom", func(c *gin.Context) {
		panic("secret database password")
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
	if got := res.Body.String(); got == "" || bytes.Contains([]byte(got), []byte("secret")) {
		t.Fatalf("response must not leak panic value: %q", got)
	}

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err == nil && entry["msg"] == "panic_recovered" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected panic_recovered log, got %s", buf.String())
	}
}

func TestFromFallsBackToDefaultLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if From(c) == nil {
		t.Fatal("expected a fallback logger, got nil")
	}
}
