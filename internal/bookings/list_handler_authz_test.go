package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

// TestListCustomerBookingsUnauthenticated covers the branch where no
// authentication middleware ran (or failed to set identity).
func TestListCustomerBookingsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Deliberately do not set the identity context keys.
	router.GET("/bookings", NewHandler(&mockBookingService{}).ListCustomerBookings)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/bookings", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

// TestListCustomerBookingsAdminForbidden documents that the customer
// history endpoint is customer-only: admins are rejected here even though
// they can read individual bookings by ID.
func TestListCustomerBookingsAdminForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.GET("/bookings", func(c *gin.Context) {
		c.Set(auth.ContextUserID, uuid.New().String())
		c.Set(auth.ContextRole, "admin")
		NewHandler(&mockBookingService{}).ListCustomerBookings(c)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/bookings", nil))

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin, got %d", response.Code)
	}
}

func TestListCustomerBookingsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.GET("/bookings", func(c *gin.Context) {
		c.Set(auth.ContextUserID, uuid.New().String())
		c.Set(auth.ContextRole, "customer")
		NewHandler(&mockBookingService{
			err: errNoRows,
		}).ListCustomerBookings(c)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/bookings", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on service failure, got %d", response.Code)
	}
}
