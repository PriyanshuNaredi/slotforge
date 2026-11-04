package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

func setupBookingListRouter(
	role string,
	userID uuid.UUID,
	handler *Handler,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.GET("/bookings", func(c *gin.Context) {
		c.Set(auth.ContextUserID, userID.String())
		c.Set(auth.ContextRole, role)
		handler.ListCustomerBookings(c)
	})

	return router
}

func TestListCustomerBookingsRejectsInvalidStatus(t *testing.T) {
	handler := &Handler{}

	router := setupBookingListRouter(
		"customer",
		uuid.New(),
		handler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/bookings?status=invalid",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}
}

func TestListCustomerBookingsRejectsInvalidLimit(t *testing.T) {
	handler := &Handler{}

	router := setupBookingListRouter(
		"customer",
		uuid.New(),
		handler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/bookings?limit=500",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}
}

func TestListCustomerBookingsRejectsInvalidOffset(t *testing.T) {
	handler := &Handler{}

	router := setupBookingListRouter(
		"customer",
		uuid.New(),
		handler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/bookings?offset=-1",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}
}

func TestListCustomerBookingsRejectsProvider(t *testing.T) {
	handler := &Handler{}

	router := setupBookingListRouter(
		"provider",
		uuid.New(),
		handler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/bookings",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			response.Code,
		)
	}
}

func TestListCustomerBookingsSuccess(t *testing.T) {
	customerID := uuid.New()

	expected := &BookingListResult{
		Bookings: []Booking{},
		Limit:    10,
		Offset:   0,
		Total:    0,
	}

	mock := &mockBookingService{
		result: expected,
	}

	handler := NewHandler(mock)

	router := setupBookingListRouter(
		"customer",
		customerID,
		handler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/bookings?limit=10&status=confirmed",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if mock.calledCustomerID != customerID {
		t.Fatal("handler did not use authenticated customer ID")
	}

	if mock.calledInput.Limit != 10 {
		t.Errorf("expected limit 10, got %d", mock.calledInput.Limit)
	}

	if mock.calledInput.Status == nil ||
		*mock.calledInput.Status != StatusConfirmed {
		t.Error("expected confirmed status filter")
	}
}
