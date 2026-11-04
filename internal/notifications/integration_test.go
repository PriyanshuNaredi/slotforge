package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

func newNotificationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

func createNotificationTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	email := fmt.Sprintf("notification-test-%s@example.com", uuid.NewString())

	var userID uuid.UUID

	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'integration-test-hash', 'customer')
		RETURNING id
	`, email).Scan(&userID)

	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			userID,
		)
		if err != nil {
			t.Errorf("clean up test user %s: %v", userID, err)
		}
	})

	return userID
}

func createTestNotification(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
	title string,
) uuid.UUID {
	t.Helper()

	var notificationID uuid.UUID

	err := pool.QueryRow(context.Background(), `
		INSERT INTO notifications (user_id, type, title, message)
		VALUES ($1, 'test_notification', $2, 'Integration test notification')
		RETURNING id
	`, userID, title).Scan(&notificationID)

	if err != nil {
		t.Fatalf("create test notification: %v", err)
	}

	return notificationID
}

func newNotificationTestRouter(userID uuid.UUID, handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	group := router.Group("/api/v1")

	group.Use(func(c *gin.Context) {
		c.Set(auth.ContextUserID, userID.String())
		c.Next()
	})

	handler.RegisterRoutes(group)

	return router
}

func performNotificationRequest(
	router http.Handler,
	method string,
	path string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	return recorder
}

func TestNotificationAPIIntegration(t *testing.T) {
	pool := newNotificationTestPool(t)

	userA := createNotificationTestUser(t, pool)
	userB := createNotificationTestUser(t, pool)

	// Create three notifications for user A.
	firstID := createTestNotification(t, pool, userA, "First notification")
	secondID := createTestNotification(t, pool, userA, "Second notification")
	createTestNotification(t, pool, userA, "Third notification")

	// Create one notification for user B.
	userBNotificationID := createTestNotification(
		t, pool, userB, "User B notification",
	)

	// Mark the first notification as already read.
	_, err := pool.Exec(context.Background(), `
		UPDATE notifications
		SET read_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, firstID, userA)

	if err != nil {
		t.Fatalf("mark first notification as read: %v", err)
	}

	repo := NewRepository(pool)
	service := NewService(repo)
	handler := NewHandler(service)

	routerA := newNotificationTestRouter(userA, handler)
	routerB := newNotificationTestRouter(userB, handler)

	t.Run("list notifications with pagination", func(t *testing.T) {
		response := performNotificationRequest(
			routerA,
			http.MethodGet,
			"/api/v1/notifications?limit=2&offset=0",
		)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s",
				response.Code, response.Body.String())
		}

		var result NotificationListResult

		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode notification response: %v", err)
		}

		if result.Total != 3 {
			t.Errorf("expected total 3, got %d", result.Total)
		}

		if result.Limit != 2 || result.Offset != 0 {
			t.Errorf("unexpected pagination: limit=%d offset=%d",
				result.Limit, result.Offset)
		}

		if len(result.Notifications) != 2 {
			t.Fatalf("expected 2 notifications, got %d",
				len(result.Notifications))
		}

		for _, notification := range result.Notifications {
			if notification.UserID != userA {
				t.Errorf("notification belongs to another user: %s",
					notification.UserID)
			}
		}
	})

	t.Run("unread count is user-specific", func(t *testing.T) {
		response := performNotificationRequest(
			routerA,
			http.MethodGet,
			"/api/v1/notifications/unread-count",
		)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", response.Code)
		}

		var body struct {
			UnreadCount int64 `json:"unread_count"`
		}

		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode unread count: %v", err)
		}

		if body.UnreadCount != 2 {
			t.Errorf("expected 2 unread notifications, got %d",
				body.UnreadCount)
		}
	})

	t.Run("mark notification as read", func(t *testing.T) {
		path := fmt.Sprintf(
			"/api/v1/notifications/%s/read",
			secondID,
		)

		response := performNotificationRequest(
			routerA,
			http.MethodPatch,
			path,
		)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s",
				response.Code, response.Body.String())
		}

		// Marking the same notification again must also succeed.
		response = performNotificationRequest(
			routerA,
			http.MethodPatch,
			path,
		)

		if response.Code != http.StatusOK {
			t.Errorf("expected repeated mark-as-read to succeed, got %d",
				response.Code)
		}
	})

	t.Run("mark all notifications as read", func(t *testing.T) {
		response := performNotificationRequest(
			routerA,
			http.MethodPatch,
			"/api/v1/notifications/read-all",
		)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s",
				response.Code, response.Body.String())
		}

		var body struct {
			UpdatedCount int64 `json:"updated_count"`
		}

		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode mark-all response: %v", err)
		}

		// Only the third notification should remain unread.
		// The second was already marked as read above.
		if body.UpdatedCount != 1 {
			t.Errorf("expected 1 updated notification, got %d",
				body.UpdatedCount)
		}

		count, err := service.CountUnread(context.Background(), userA)
		if err != nil {
			t.Fatalf("count unread notifications: %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 unread notifications, got %d", count)
		}

		// User B's notification must remain unread.
		userBCount, err := service.CountUnread(context.Background(), userB)
		if err != nil {
			t.Fatalf("count user B notifications: %v", err)
		}

		if userBCount != 1 {
			t.Errorf("expected user B to still have 1 unread notification, got %d",
				userBCount)
		}
	})

	t.Run("cannot mark another user's notification as read", func(t *testing.T) {
		path := fmt.Sprintf(
			"/api/v1/notifications/%s/read",
			userBNotificationID,
		)

		response := performNotificationRequest(
			routerA,
			http.MethodPatch,
			path,
		)

		if response.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d: %s",
				response.Code, response.Body.String())
		}

		// Verify that user B's notification is still unread.
		notification, err := repo.GetByID(
			context.Background(),
			userB,
			userBNotificationID,
		)

		if err != nil {
			t.Fatalf("retrieve user B notification: %v", err)
		}

		if notification.ReadAt != nil {
			t.Error("user B's notification should remain unread")
		}
	})

	t.Run("invalid notification ID returns 400", func(t *testing.T) {
		response := performNotificationRequest(
			routerA,
			http.MethodPatch,
			"/api/v1/notifications/invalid-id/read",
		)

		if response.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", response.Code)
		}
	})

	t.Run("invalid pagination returns 400", func(t *testing.T) {
		response := performNotificationRequest(
			routerA,
			http.MethodGet,
			"/api/v1/notifications?limit=101",
		)

		if response.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", response.Code)
		}

		response = performNotificationRequest(
			routerA,
			http.MethodGet,
			"/api/v1/notifications?offset=-1",
		)

		if response.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for negative offset, got %d",
				response.Code)
		}
	})

	t.Run("user B can list only their own notifications", func(t *testing.T) {
		response := performNotificationRequest(
			routerB,
			http.MethodGet,
			"/api/v1/notifications",
		)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", response.Code)
		}

		var result NotificationListResult

		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode notification response: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected 1 notification for user B, got %d",
				result.Total)
		}

		for _, notification := range result.Notifications {
			if notification.UserID != userB {
				t.Errorf("user B received another user's notification")
			}
		}
	})
}
