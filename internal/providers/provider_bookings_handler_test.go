package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

// Test-only secret, never used by the application.
const dashboardTestSecret = "providers-dashboard-test-secret-0123456789"

type dashboardResponse struct {
	Bookings []map[string]any `json:"bookings"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	Total    int64            `json:"total"`
	Error    string           `json:"error"`
}

// newDashboardRouter wires the same chain as cmd/api/main.go:
// authentication middleware, provider-role middleware, handler routes.
func newDashboardRouter(
	t *testing.T,
	service *Service,
) (*gin.Engine, *auth.TokenManager) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	manager, err := auth.NewTokenManager(
		dashboardTestSecret,
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	router := gin.New()

	api := router.Group("/api/v1")

	protected := api.Group("")
	protected.Use(auth.AuthenticationMiddleware(manager))

	providerRoutes := protected.Group("")
	providerRoutes.Use(auth.RequireRoles("provider"))

	NewHandler(service).RegisterRoutes(providerRoutes)

	return router, manager
}

func doDashboardRequest(
	t *testing.T,
	router *gin.Engine,
	token string,
	query string,
) (*httptest.ResponseRecorder, dashboardResponse) {
	t.Helper()

	path := "/api/v1/providers/bookings"

	if query != "" {
		path += "?" + query
	}

	request := httptest.NewRequest(http.MethodGet, path, nil)

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	var parsed dashboardResponse

	if response.Body.Len() > 0 {
		if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("decode response %q: %v", response.Body.String(), err)
		}
	}

	return response, parsed
}

func TestProviderBookingsHandlerHappyPath(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	start := time.Now().UTC().Add(150 * time.Hour).Truncate(time.Minute)

	createDashboardBooking(t, pool, service, providerID, customerID, start, "confirmed")

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerID, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	response, body := doDashboardRequest(t, router, token, "")

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body.Total != 1 || len(body.Bookings) != 1 {
		t.Fatalf("expected 1 booking, total=%d returned=%d", body.Total, len(body.Bookings))
	}

	booking := body.Bookings[0]

	if email, exists := booking["customer_email"]; !exists || email == "" {
		t.Error("expected a non-empty customer_email in response")
	}

	if booking["customer_id"] != customerID.String() {
		t.Errorf("expected customer %s, got %v", customerID, booking["customer_id"])
	}

	for _, key := range []string{
		"id", "slot_id", "status", "slot_starts_at", "slot_ends_at", "created_at",
	} {
		if _, exists := booking[key]; !exists {
			t.Errorf("expected key %q in booking JSON", key)
		}
	}

	if body.Limit != 20 || body.Offset != 0 {
		t.Errorf("expected default pagination metadata 20/0, got %d/%d", body.Limit, body.Offset)
	}
}

func TestProviderBookingsHandlerAuthentication(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	router, manager := newDashboardRouter(t, service)

	createDashboardProvider(t, pool, service)

	t.Run("missing token returns 401", func(t *testing.T) {
		response, _ := doDashboardRequest(t, router, "", "")

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", response.Code)
		}
	})

	t.Run("customer role returns 403", func(t *testing.T) {
		token, err := manager.GenerateAccessToken(uuid.New(), "customer")
		if err != nil {
			t.Fatalf("generate customer token: %v", err)
		}

		response, _ := doDashboardRequest(t, router, token, "")

		if response.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", response.Code)
		}
	})

	t.Run("garbage token returns 401", func(t *testing.T) {
		response, _ := doDashboardRequest(t, router, "not.a.jwt", "")

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", response.Code)
		}
	})
}

func TestProviderBookingsHandlerValidation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerID, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	validFrom := time.Now().UTC().Add(150 * time.Hour).Format(time.RFC3339)
	validTo := time.Now().UTC().Add(160 * time.Hour).Format(time.RFC3339)

	cases := []struct {
		name          string
		query         string
		expectedError string
	}{
		{"invalid status", "status=bogus", ErrInvalidBookingStatus.Error()},
		{"invalid from", "from=yesterday", "from must be RFC3339 format"},
		{"invalid to", "to=tomorrow", "to must be RFC3339 format"},
		{"invalid limit", "limit=abc", "invalid limit"},
		{"invalid offset", "offset=abc", "invalid offset"},
		{"limit above 100", "limit=101", ErrInvalidLimit.Error()},
		{"negative offset", "offset=-1", ErrInvalidOffset.Error()},
		{
			"from equals to",
			"from=" + validFrom + "&to=" + validFrom,
			ErrInvalidDateRange.Error(),
		},
		{
			"from after to",
			"from=" + validTo + "&to=" + validFrom,
			ErrInvalidDateRange.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, body := doDashboardRequest(t, router, token, tc.query)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}

			if body.Error != tc.expectedError {
				t.Errorf("expected error %q, got %q", tc.expectedError, body.Error)
			}
		})
	}

	t.Run("zero limit defaults to 20", func(t *testing.T) {
		response, body := doDashboardRequest(t, router, token, "limit=0")

		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		if body.Limit != 20 {
			t.Errorf("expected limit 20, got %d", body.Limit)
		}
	})
}

func TestProviderBookingsHandlerEmptyResults(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerID, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	response, body := doDashboardRequest(
		t, router, token,
		fmt.Sprintf("from=%s&to=%s",
			time.Now().UTC().Add(150*time.Hour).Format(time.RFC3339),
			time.Now().UTC().Add(160*time.Hour).Format(time.RFC3339),
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	// A nil slice decodes to nil; an empty JSON array stays non-nil.
	if body.Bookings == nil {
		t.Fatalf("expected bookings to serialize as [], got %s", response.Body.String())
	}

	if len(body.Bookings) != 0 || body.Total != 0 {
		t.Errorf("expected empty result set with total 0, got %d bookings total=%d",
			len(body.Bookings), body.Total)
	}
}

func TestProviderBookingsHandlerIgnoresClientProviderID(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerA := createDashboardProvider(t, pool, service)
	providerB := createDashboardProvider(t, pool, service)

	customerA := createDashboardCustomer(t, pool)
	customerB := createDashboardCustomer(t, pool)

	base := time.Now().UTC().Add(180 * time.Hour).Truncate(time.Minute)

	createDashboardBooking(t, pool, service, providerA, customerA, base, "confirmed")
	createDashboardBooking(t, pool, service, providerB, customerB, base.Add(time.Hour), "confirmed")

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerA, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	// The endpoint has no provider_id parameter; passing one must not
	// widen or change the result set.
	response, body := doDashboardRequest(
		t, router, token,
		"provider_id="+providerB.String(),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body.Total != 1 {
		t.Fatalf("expected only provider A's booking, got total %d", body.Total)
	}

	if body.Bookings[0]["customer_id"] != customerA.String() {
		t.Error("provider A received a booking belonging to another provider")
	}
}
