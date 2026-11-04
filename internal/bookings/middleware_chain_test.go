package bookings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

// Test-only secrets, never used by the application.
const (
	wiredTestSecret  = "bookings-wired-test-secret-0123456789"
	wiredOtherSecret = "bookings-other-test-secret-012345678"
)

// The middleware chain mirrors cmd/api/main.go: public discovery routes,
// then a protected group behind AuthenticationMiddleware, with the
// booking routes additionally restricted by role.
func newWiredRouter(t *testing.T, mock *mockBookingService) (*gin.Engine, *auth.TokenManager) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	manager, err := auth.NewTokenManager(
		wiredTestSecret,
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	router := gin.New()

	api := router.Group("/api/v1")

	NewHandler(mock).RegisterPublicRoutes(api)

	protected := api.Group("")
	protected.Use(auth.AuthenticationMiddleware(manager))

	NewHandler(mock).RegisterRoutes(protected)

	return router, manager
}

func doWiredRequest(
	t *testing.T,
	router *gin.Engine,
	method string,
	path string,
	payload any,
	token string,
) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	var body []byte

	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = raw
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	var parsed map[string]any

	if response.Body.Len() > 0 {
		if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("decode response %q: %v", response.Body.String(), err)
		}
	}

	return response, parsed
}

func TestMiddlewareChainUnauthenticated(t *testing.T) {
	router, _ := newWiredRouter(t, &mockBookingService{})

	response, _ := doWiredRequest(
		t, router, http.MethodPost, "/api/v1/bookings",
		map[string]any{"slot_id": uuid.NewString()},
		"",
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", response.Code)
	}
}

func TestMiddlewareChainInvalidTokens(t *testing.T) {
	router, _ := newWiredRouter(t, &mockBookingService{})

	// A token signed by a different manager.
	otherManager, err := auth.NewTokenManager(
		wiredOtherSecret,
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create other manager: %v", err)
	}

	foreignToken, err := otherManager.GenerateAccessToken(uuid.New(), "customer")
	if err != nil {
		t.Fatalf("generate foreign token: %v", err)
	}

	// Expired but correctly signed: must still be rejected.
	now := time.Now()

	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "slotforge",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	})

	expiredToken, err := expired.SignedString([]byte(wiredTestSecret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	cases := map[string]string{
		"garbage token":  "not.a.jwt",
		"foreign secret": foreignToken,
		"expired token":  expiredToken,
	}

	for name, token := range cases {
		response, _ := doWiredRequest(
			t, router, http.MethodPost, "/api/v1/bookings",
			map[string]any{"slot_id": uuid.NewString()},
			token,
		)

		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, response.Code)
		}
	}
}

func TestMiddlewareChainRoleEnforcement(t *testing.T) {
	router, manager := newWiredRouter(t, &mockBookingService{
		booking: &Booking{
			ID:         uuid.New(),
			SlotID:     uuid.New(),
			CustomerID: uuid.New(),
			Status:     StatusConfirmed,
		},
	})

	customerToken, err := manager.GenerateAccessToken(uuid.New(), "customer")
	if err != nil {
		t.Fatalf("generate customer token: %v", err)
	}

	providerToken, err := manager.GenerateAccessToken(uuid.New(), "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	// Customers can create bookings.
	response, _ := doWiredRequest(
		t, router, http.MethodPost, "/api/v1/bookings",
		map[string]any{"slot_id": uuid.NewString()},
		customerToken,
	)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201 for customer, got %d: %s", response.Code, response.Body.String())
	}

	// Providers are rejected on customer-only booking routes.
	response, _ = doWiredRequest(
		t, router, http.MethodPost, "/api/v1/bookings",
		map[string]any{"slot_id": uuid.NewString()},
		providerToken,
	)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for provider on POST /bookings, got %d", response.Code)
	}

	response, _ = doWiredRequest(
		t, router, http.MethodGet, "/api/v1/bookings",
		nil,
		providerToken,
	)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for provider on GET /bookings, got %d", response.Code)
	}

	// GET /bookings/:id is open to any authenticated role; ownership
	// is enforced inside the service.
	response, _ = doWiredRequest(
		t, router, http.MethodGet,
		"/api/v1/bookings/"+uuid.New().String(),
		nil,
		providerToken,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 for provider on GET /bookings/:id, got %d", response.Code)
	}
}

func TestMiddlewareChainPublicRoutesNeedNoToken(t *testing.T) {
	router, _ := newWiredRouter(t, &mockBookingService{
		searchResult: &SlotSearchResult{
			Slots: []AvailableSlot{}, Limit: 20, Offset: 0, Total: 0,
		},
	})

	response, _ := doWiredRequest(t, router, http.MethodGet, "/api/v1/slots", nil, "")

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 on public slot listing, got %d", response.Code)
	}
}
