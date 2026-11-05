package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newAuthHandlerRouter builds the real handler stack on a real Gin router
// with a real service, repository, and PostgreSQL pool.
func newAuthHandlerRouter(
	t *testing.T,
) (*gin.Engine, *Service, *pgxpool.Pool) {
	t.Helper()

	service, pool := newTestService(t)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	api := router.Group("/api/v1")

	handler := NewHandler(service)
	handler.RegisterRoutes(api)

	protected := api.Group("")
	protected.Use(AuthenticationMiddleware(service.tokens))
	handler.RegisterProtectedRoutes(protected)

	return router, service, pool
}

type authResponseBody struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	User         struct {
		ID           string `json:"id"`
		Email        string `json:"email"`
		Role         string `json:"role"`
		PasswordHash string `json:"password_hash"`
	} `json:"user"`
	Error string `json:"error"`
}

func postJSON(
	t *testing.T,
	router *gin.Engine,
	path string,
	payload any,
) (*httptest.ResponseRecorder, authResponseBody) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	var parsed authResponseBody

	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}

	return response, parsed
}

func mustParseUserID(t *testing.T, raw string) uuid.UUID {
	t.Helper()

	userID, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("parse user ID %q: %v", raw, err)
	}

	return userID
}

func postAuthJSON(
	t *testing.T,
	router *gin.Engine,
	path string,
	payload any,
	accessToken string,
) (*httptest.ResponseRecorder, authResponseBody) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	var parsed authResponseBody

	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}

	return response, parsed
}

func TestRegisterHandler(t *testing.T) {
	router, service, pool := newAuthHandlerRouter(t)

	email := uniqueEmail()

	response, body := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}

	userID := mustParseUserID(t, body.User.ID)
	deleteTestUser(t, pool, userID)

	if body.AccessToken == "" || body.RefreshToken == "" {
		t.Fatal("expected both tokens to be returned")
	}

	if body.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %q", body.TokenType)
	}

	if body.ExpiresIn != int64((15 * time.Minute).Seconds()) {
		t.Errorf("expected expires_in 900, got %d", body.ExpiresIn)
	}

	if body.User.Email != email {
		t.Errorf("expected email %q, got %q", email, body.User.Email)
	}

	if body.User.Role != "customer" {
		t.Errorf("expected customer role, got %q", body.User.Role)
	}

	if body.User.PasswordHash != "" {
		t.Error("password hash must not be exposed in the registration response")
	}

	if _, err := service.tokens.ValidateAccessToken(body.AccessToken); err != nil {
		t.Errorf("access token should validate: %v", err)
	}
}

func TestRegisterHandlerValidationErrors(t *testing.T) {
	router, _, _ := newAuthHandlerRouter(t)

	cases := []struct {
		name     string
		email    string
		password string
	}{
		{"invalid email", "not-an-email", "correct horse battery"},
		{"missing email", "", "correct horse battery"},
		{"short password", "user@example.com", "only10chars"},
		{"missing password", "user@example.com", ""},
		{
			// 40 runes but 80 bytes: passes rune validation,
			// fails the byte limit in the service.
			"password over 72 bytes",
			"user@example.com",
			strings.Repeat("é", 40),
		},
	}

	for _, tc := range cases {
		response, body := postJSON(t, router, "/api/v1/auth/register", map[string]string{
			"email":    tc.email,
			"password": tc.password,
		})

		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", tc.name, response.Code)
		}

		if body.Error == "" {
			t.Errorf("%s: expected an error message", tc.name)
		}
	}
}

func TestRegisterHandlerDuplicateEmail(t *testing.T) {
	router, _, pool := newAuthHandlerRouter(t)

	email := uniqueEmail()

	first, firstBody := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	if first.Code != http.StatusCreated {
		t.Fatalf("expected 201 on first registration, got %d", first.Code)
	}

	deleteTestUser(t, pool, mustParseUserID(t, firstBody.User.ID))

	second, body := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    strings.ToUpper(email),
		"password": "correct horse battery",
	})

	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate email, got %d", second.Code)
	}

	if body.Error != ErrEmailTaken.Error() {
		t.Errorf("expected %q, got %q", ErrEmailTaken.Error(), body.Error)
	}
}

func TestLoginHandler(t *testing.T) {
	router, _, pool := newAuthHandlerRouter(t)

	email := uniqueEmail()

	_, registered := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	deleteTestUser(t, pool, mustParseUserID(t, registered.User.ID))

	response, body := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body.AccessToken == "" || body.RefreshToken == "" {
		t.Fatal("expected tokens on successful login")
	}

	if body.User.PasswordHash != "" {
		t.Error("password hash must not be exposed in the login response")
	}
}

func TestLoginHandlerRejectsBadCredentials(t *testing.T) {
	router, _, pool := newAuthHandlerRouter(t)

	email := uniqueEmail()

	_, registered := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	deleteTestUser(t, pool, mustParseUserID(t, registered.User.ID))

	cases := []struct {
		name     string
		email    string
		password string
	}{
		{"wrong password", email, "wrong horse battery"},
		{"unknown email", "ghost-" + uniqueEmail(), "correct horse battery"},
	}

	for _, tc := range cases {
		response, body := postJSON(t, router, "/api/v1/auth/login", map[string]string{
			"email":    tc.email,
			"password": tc.password,
		})

		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", tc.name, response.Code)
		}

		if body.Error != "invalid email or password" {
			t.Errorf("%s: expected generic error, got %q", tc.name, body.Error)
		}
	}
}

func TestLoginHandlerRejectsInactiveUser(t *testing.T) {
	router, _, pool := newAuthHandlerRouter(t)

	email := uniqueEmail()

	_, registered := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	userID := mustParseUserID(t, registered.User.ID)
	deleteTestUser(t, pool, userID)

	_, err := pool.Exec(context.Background(), `
		UPDATE users SET is_active = FALSE WHERE id = $1
	`, userID)
	if err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	response, _ := postJSON(t, router, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": "correct horse battery",
	})

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for inactive user, got %d", response.Code)
	}
}

func TestRefreshHandler(t *testing.T) {
	router, _, pool := newAuthHandlerRouter(t)

	_, registered := postJSON(t, router, "/api/v1/auth/register", map[string]string{
		"email":    uniqueEmail(),
		"password": "correct horse battery",
	})

	deleteTestUser(t, pool, mustParseUserID(t, registered.User.ID))

	response, refreshed := postJSON(t, router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": registered.RefreshToken,
	})

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if refreshed.RefreshToken == registered.RefreshToken {
		t.Error("expected a rotated refresh token")
	}

	if refreshed.AccessToken == "" {
		t.Error("expected a new access token")
	}

	// Reusing the rotated token must fail.
	reuse, _ := postJSON(t, router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": registered.RefreshToken,
	})

	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on refresh token reuse, got %d", reuse.Code)
	}
}

func TestRefreshHandlerValidation(t *testing.T) {
	router, _, _ := newAuthHandlerRouter(t)

	missing, _ := postJSON(t, router, "/api/v1/auth/refresh", map[string]string{})
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without refresh_token, got %d", missing.Code)
	}

	garbage, _ := postJSON(t, router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": "not-a-real-token",
	})
	if garbage.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown refresh token, got %d", garbage.Code)
	}
}

func TestLogoutHandler(t *testing.T) {
	router, service, pool := newAuthHandlerRouter(t)

	ctx := context.Background()

	registered, err := service.Register(ctx, uniqueEmail(), "correct horse battery")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	deleteTestUser(t, pool, registered.User.ID)

	// Without an access token the endpoint must refuse.
	unauthenticated, _ := postJSON(t, router, "/api/v1/auth/logout", map[string]string{
		"refresh_token": registered.RefreshToken,
	})
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without access token, got %d", unauthenticated.Code)
	}

	// The token must survive a refused logout.
	if _, err := service.Refresh(ctx, registered.RefreshToken); err != nil {
		t.Fatalf("token must still be usable after refused logout: %v", err)
	}

	response, body := postAuthJSON(
		t,
		router,
		"/api/v1/auth/logout",
		map[string]string{"refresh_token": registered.RefreshToken},
		registered.AccessToken,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var okBody struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &okBody); err != nil {
		t.Fatalf("decode logout response: %v", err)
	}
	if !okBody.OK {
		t.Errorf("expected ok=true, got %s", response.Body.String())
	}
	if body.Error != "" {
		t.Errorf("unexpected error field: %q", body.Error)
	}

	// Reuse of the revoked token must be rejected.
	revoked, _ := postJSON(t, router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": registered.RefreshToken,
	})
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for refresh after logout, got %d", revoked.Code)
	}

	// Repeated logout stays a success.
	repeat, _ := postAuthJSON(
		t,
		router,
		"/api/v1/auth/logout",
		map[string]string{"refresh_token": registered.RefreshToken},
		registered.AccessToken,
	)
	if repeat.Code != http.StatusOK {
		t.Fatalf("expected repeated logout to return 200, got %d", repeat.Code)
	}
}
