// Package logging provides structured JSON logging: a slog logger, a
// request-ID middleware, an access-log middleware, and a panic-recovery
// middleware. Every log line is a single JSON object on stdout.
package logging

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// authUserIDKey matches auth.ContextUserID without importing the auth
// package, which would create an import cycle.
const authUserIDKey = "userID"

type contextKey int

const (
	requestIDKey contextKey = iota
	loggerKey
)

// New returns the application logger: JSON to stdout at info level.
func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// RequestID assigns an X-Request-ID (honouring a sane inbound one) and
// stores it in the gin context, the request context, and the response
// header. Inbound IDs must be short and alphanumeric so clients cannot
// pollute logs or the response header with arbitrary bytes.
var inboundRequestID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !inboundRequestID.MatchString(id) {
			id = uuid.NewString()
		}

		c.Set("request_id", id)
		c.Writer.Header().Set("X-Request-ID", id)
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), requestIDKey, id),
		)
		c.Next()
	}
}

// RequestIDFrom returns the request ID recorded by RequestID, if any.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithLogger stores a request-scoped logger with the request ID attached.
func WithLogger(c *gin.Context, logger *slog.Logger) {
	c.Set("logger", logger.With(slog.String("request_id", RequestIDFrom(c.Request.Context()))))
}

// From returns the request-scoped logger, or the default logger when the
// middleware has not run (e.g. in tests).
func From(c *gin.Context) *slog.Logger {
	if logger, ok := c.Get("logger"); ok {
		if l, ok := logger.(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// Access emits one JSON line per request after it completes. It also
// installs the request-scoped logger so handlers can log events with the
// request ID attached.
func Access(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		WithLogger(c, logger)
		c.Next()

		userID, _ := c.Get(authUserIDKey)
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
		}
		if id, ok := userID.(string); ok {
			attrs = append(attrs, slog.String("user_id", id))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}
		From(c).Info("http_request", attrs...)
	}
}

// Recovery converts panics into structured error logs and a 500 response.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic_recovered",
					slog.String("request_id", RequestIDFrom(c.Request.Context())),
					slog.Any("panic", rec),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
