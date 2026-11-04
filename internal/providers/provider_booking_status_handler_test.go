package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func doBookingStatusRequest(
	t *testing.T,
	router *gin.Engine,
	token string,
	bookingID string,
	body string,
) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/providers/bookings/"+bookingID+"/status",
		strings.NewReader(body),
	)
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

func TestProviderBookingStatusHandlerHappyPath(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	bookingID := createStatusTestBooking(
		t,
		pool,
		service,
		providerID,
		customerID,
		"confirmed",
		time.Now().UTC().Add(300*time.Hour).Truncate(time.Minute),
	)

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerID, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	response, body := doBookingStatusRequest(
		t,
		router,
		token,
		bookingID.String(),
		`{"status":"completed"}`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body["id"] != bookingID.String() {
		t.Errorf("unexpected booking ID: %v", body["id"])
	}

	if body["status"] != "completed" {
		t.Errorf("expected completed status, got %v", body["status"])
	}

	if body["customer_email"] == nil || body["customer_email"] == "" {
		t.Error("expected customer_email in response")
	}

	if got := getBookingStatus(t, pool, bookingID); got != "completed" {
		t.Errorf("expected database status completed, got %q", got)
	}
}

func TestProviderBookingStatusHandlerAuthentication(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	bookingID := createStatusTestBooking(
		t,
		pool,
		service,
		providerID,
		customerID,
		"confirmed",
		time.Now().UTC().Add(310*time.Hour).Truncate(time.Minute),
	)

	router, manager := newDashboardRouter(t, service)

	t.Run("missing token returns 401", func(t *testing.T) {
		response, _ := doBookingStatusRequest(
			t, router, "", bookingID.String(), `{"status":"completed"}`,
		)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", response.Code)
		}
	})

	t.Run("customer role returns 403", func(t *testing.T) {
		token, err := manager.GenerateAccessToken(uuid.New(), "customer")
		if err != nil {
			t.Fatalf("generate customer token: %v", err)
		}

		response, _ := doBookingStatusRequest(
			t, router, token, bookingID.String(), `{"status":"completed"}`,
		)

		if response.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", response.Code)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		response, _ := doBookingStatusRequest(
			t, router, "not.a.jwt", bookingID.String(), `{"status":"completed"}`,
		)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", response.Code)
		}
	})
}

func TestProviderBookingStatusHandlerValidation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerID, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	cases := []struct {
		name       string
		bookingID  string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid booking UUID",
			bookingID:  "not-a-uuid",
			body:       `{"status":"completed"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing status",
			bookingID:  uuid.NewString(),
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			bookingID:  uuid.NewString(),
			body:       `not-json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid requested status",
			bookingID:  uuid.NewString(),
			body:       `{"status":"pending"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown requested status",
			bookingID:  uuid.NewString(),
			body:       `{"status":"approved"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, _ := doBookingStatusRequest(
				t, router, token, tc.bookingID, tc.body,
			)

			if response.Code != tc.wantStatus {
				t.Fatalf(
					"expected %d, got %d: %s",
					tc.wantStatus,
					response.Code,
					response.Body.String(),
				)
			}
		})
	}
}

func TestProviderBookingStatusHandlerBusinessErrors(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerA := createDashboardProvider(t, pool, service)
	providerB := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	base := time.Now().UTC().Add(320 * time.Hour).Truncate(time.Minute)

	otherProviderBooking := createStatusTestBooking(
		t, pool, service, providerB, customerID,
		"confirmed", base,
	)

	completedBooking := createStatusTestBooking(
		t, pool, service, providerA, customerID,
		"completed", base.Add(time.Hour),
	)

	confirmedBooking := createStatusTestBooking(
		t, pool, service, providerA, customerID,
		"confirmed", base.Add(2*time.Hour),
	)

	router, manager := newDashboardRouter(t, service)

	token, err := manager.GenerateAccessToken(providerA, "provider")
	if err != nil {
		t.Fatalf("generate provider token: %v", err)
	}

	t.Run("booking belonging to another provider returns 404", func(t *testing.T) {
		response, _ := doBookingStatusRequest(
			t, router, token, otherProviderBooking.String(),
			`{"status":"completed"}`,
		)

		if response.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", response.Code, response.Body.String())
		}

		if got := getBookingStatus(t, pool, otherProviderBooking); got != "confirmed" {
			t.Errorf("other provider's booking was modified: %q", got)
		}
	})

	t.Run("terminal booking returns 409", func(t *testing.T) {
		response, _ := doBookingStatusRequest(
			t, router, token, completedBooking.String(),
			`{"status":"cancelled"}`,
		)

		if response.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("unknown booking returns 404", func(t *testing.T) {
		response, _ := doBookingStatusRequest(
			t, router, token, uuid.NewString(),
			`{"status":"completed"}`,
		)

		if response.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", response.Code)
		}
	})

	t.Run("valid update succeeds after rejected update", func(t *testing.T) {
		response, body := doBookingStatusRequest(
			t, router, token, confirmedBooking.String(),
			`{"status":"no_show"}`,
		)

		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		if body["status"] != "no_show" {
			t.Errorf("expected no_show, got %v", body["status"])
		}
	})
}
