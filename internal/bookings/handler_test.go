package bookings

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

func newBookingHandlerRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Inject identity like AuthenticationMiddleware would.
	router.Use(func(c *gin.Context) {
		if userID := c.GetHeader("X-Test-User"); userID != "" {
			c.Set(auth.ContextUserID, userID)
			c.Set(auth.ContextRole, c.GetHeader("X-Test-Role"))
		}

		c.Next()
	})

	handler.RegisterRoutes(router.Group(""))

	return router
}

func doBookingRequest(
	t *testing.T,
	router *gin.Engine,
	method string,
	path string,
	payload any,
	userID string,
	role string,
) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	var body *bytes.Reader

	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}

	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")

	if userID != "" {
		request.Header.Set("X-Test-User", userID)
		request.Header.Set("X-Test-Role", role)
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

func TestCreateBookingHandlerRequiresAuthentication(t *testing.T) {
	router := newBookingHandlerRouter(NewHandler(&mockBookingService{}))

	response, _ := doBookingRequest(
		t, router, http.MethodPost, "/bookings",
		map[string]any{"slot_id": uuid.NewString()},
		"", "",
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without identity, got %d", response.Code)
	}
}

func TestCreateBookingHandlerValidation(t *testing.T) {
	router := newBookingHandlerRouter(NewHandler(&mockBookingService{}))

	userID := uuid.New().String()

	response, _ := doBookingRequest(
		t, router, http.MethodPost, "/bookings",
		nil,
		userID, "customer",
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", response.Code)
	}

	response, _ = doBookingRequest(
		t, router, http.MethodPost, "/bookings",
		map[string]any{"notes": "no slot id"},
		userID, "customer",
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without slot_id, got %d", response.Code)
	}
}

func TestCreateBookingHandlerMapsRepositoryErrors(t *testing.T) {
	customerID := uuid.New()

	cases := []struct {
		name       string
		serviceErr error
		expected   int
	}{
		{"slot not found", ErrSlotNotFound, http.StatusNotFound},
		{"slot closed", ErrSlotClosed, http.StatusConflict},
		{"slot booked", ErrSlotBooked, http.StatusConflict},
		{"unexpected error", errors.New("database down"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockBookingService{bookingErr: tc.serviceErr}

			router := newBookingHandlerRouter(NewHandler(mock))

			response, _ := doBookingRequest(
				t, router, http.MethodPost, "/bookings",
				map[string]any{"slot_id": uuid.NewString()},
				customerID.String(), "customer",
			)

			if response.Code != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, response.Code)
			}
		})
	}
}

func TestCreateBookingHandlerUsesAuthenticatedCustomer(t *testing.T) {
	customerID := uuid.New()

	bookingID := uuid.New()

	mock := &mockBookingService{
		booking: &Booking{
			ID:         bookingID,
			SlotID:     uuid.New(),
			CustomerID: customerID,
			Status:     StatusConfirmed,
		},
	}

	router := newBookingHandlerRouter(NewHandler(mock))

	slotID := uuid.New()

	response, body := doBookingRequest(
		t, router, http.MethodPost, "/bookings",
		map[string]any{"slot_id": slotID.String()},
		customerID.String(), "customer",
	)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}

	if mock.calledBookingCustomerID != customerID {
		t.Error("handler must pass the authenticated customer ID to the service")
	}

	if mock.calledCreateInput.SlotID != slotID {
		t.Error("handler must pass the requested slot ID to the service")
	}

	if body["id"] != bookingID.String() {
		t.Errorf("expected booking id %s in response, got %v", bookingID, body["id"])
	}

	if body["status"] != string(StatusConfirmed) {
		t.Errorf("expected status confirmed, got %v", body["status"])
	}
}

func TestGetBookingHandler(t *testing.T) {
	bookingID := uuid.New()
	customerID := uuid.New()

	cases := []struct {
		name      string
		role      string
		expectIsA bool
	}{
		{"customer is not admin", "customer", false},
		{"admin flag is forwarded", "admin", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockBookingService{
				detail: &BookingDetail{
					ID:         bookingID,
					CustomerID: customerID,
					Status:     StatusConfirmed,
				},
			}

			router := newBookingHandlerRouter(NewHandler(mock))

			response, body := doBookingRequest(
				t, router, http.MethodGet,
				"/bookings/"+bookingID.String(),
				nil,
				customerID.String(), tc.role,
			)

			if response.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
			}

			if mock.calledIsAdmin != tc.expectIsA {
				t.Errorf("expected isAdmin %v, got %v", tc.expectIsA, mock.calledIsAdmin)
			}

			if mock.calledGetID != bookingID {
				t.Error("handler must pass the parsed booking ID")
			}

			if body["id"] != bookingID.String() {
				t.Errorf("expected id %s, got %v", bookingID, body["id"])
			}
		})
	}
}

func TestGetBookingHandlerInvalidUUID(t *testing.T) {
	router := newBookingHandlerRouter(NewHandler(&mockBookingService{}))

	response, _ := doBookingRequest(
		t, router, http.MethodGet, "/bookings/not-a-uuid",
		nil,
		uuid.New().String(), "customer",
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestGetBookingHandlerNotFound(t *testing.T) {
	mock := &mockBookingService{bookingErr: errNoRows}

	router := newBookingHandlerRouter(NewHandler(mock))

	response, body := doBookingRequest(
		t, router, http.MethodGet, "/bookings/"+uuid.New().String(),
		nil,
		uuid.New().String(), "customer",
	)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}

	if body["error"] == "" {
		t.Error("expected an error message in the response")
	}
}
