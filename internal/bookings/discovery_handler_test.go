package bookings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newDiscoveryRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler.RegisterPublicRoutes(router.Group(""))

	return router
}

func doGet(t *testing.T, router *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)

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

func TestSearchSlotsHandlerRejectsInvalidProviderID(t *testing.T) {
	router := newDiscoveryRouter(NewHandler(&mockBookingService{}))

	response, _ := doGet(t, router, "/slots?provider_id=not-a-uuid")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestSearchSlotsHandlerRejectsInvalidDates(t *testing.T) {
	router := newDiscoveryRouter(NewHandler(&mockBookingService{}))

	cases := map[string]string{
		"invalid from": "/slots?from=yesterday",
		"invalid to":   "/slots?to=2026-13-45",
		"from equals to": "/slots?from=" +
			time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339) +
			"&to=" +
			time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"from after to": "/slots?from=" +
			time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC).Format(time.RFC3339) +
			"&to=" +
			time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}

	for name, path := range cases {
		response, _ := doGet(t, router, path)

		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, response.Code)
		}
	}
}

func TestSearchSlotsHandlerRejectsInvalidPagination(t *testing.T) {
	router := newDiscoveryRouter(NewHandler(&mockBookingService{}))

	cases := map[string]string{
		"limit zero":      "/slots?limit=0",
		"limit too big":   "/slots?limit=101",
		"limit letters":   "/slots?limit=many",
		"offset negative": "/slots?offset=-1",
		"offset letters":  "/slots?offset=x",
	}

	for name, path := range cases {
		response, _ := doGet(t, router, path)

		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, response.Code)
		}
	}
}

func TestSearchSlotsHandlerForwardsFilters(t *testing.T) {
	providerID := uuid.New()

	from := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC)

	mock := &mockBookingService{
		searchResult: &SlotSearchResult{
			Slots:  []AvailableSlot{},
			Limit:  25,
			Offset: 5,
			Total:  42,
		},
	}

	router := newDiscoveryRouter(NewHandler(mock))

	path := "/slots?provider_id=" + providerID.String() +
		"&from=" + from.Format(time.RFC3339) +
		"&to=" + to.Format(time.RFC3339) +
		"&limit=25&offset=5"

	response, body := doGet(t, router, path)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if mock.calledSearchInput.ProviderID == nil ||
		*mock.calledSearchInput.ProviderID != providerID {
		t.Error("provider filter was not forwarded")
	}

	if mock.calledSearchInput.From == nil || !mock.calledSearchInput.From.Equal(from) {
		t.Error("from filter was not forwarded")
	}

	if mock.calledSearchInput.To == nil || !mock.calledSearchInput.To.Equal(to) {
		t.Error("to filter was not forwarded")
	}

	if mock.calledSearchInput.Limit != 25 || mock.calledSearchInput.Offset != 5 {
		t.Errorf("pagination not forwarded: limit=%d offset=%d",
			mock.calledSearchInput.Limit, mock.calledSearchInput.Offset)
	}

	if body["total"] != float64(42) {
		t.Errorf("expected total 42, got %v", body["total"])
	}

	if body["limit"] != float64(25) || body["offset"] != float64(5) {
		t.Errorf("expected echoed pagination, got %v", body)
	}
}

func TestSearchSlotsHandlerUsesDefaultPagination(t *testing.T) {
	mock := &mockBookingService{
		searchResult: &SlotSearchResult{
			Slots: []AvailableSlot{},
			Limit: 20, Offset: 0, Total: 0,
		},
	}

	router := newDiscoveryRouter(NewHandler(mock))

	response, _ := doGet(t, router, "/slots")

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	if mock.calledSearchInput.Limit != 20 || mock.calledSearchInput.Offset != 0 {
		t.Errorf("expected default limit 20 offset 0, got %d/%d",
			mock.calledSearchInput.Limit, mock.calledSearchInput.Offset)
	}
}

func TestGetSlotHandler(t *testing.T) {
	slotID := uuid.New()

	t.Run("invalid uuid returns 400", func(t *testing.T) {
		router := newDiscoveryRouter(NewHandler(&mockBookingService{}))

		response, _ := doGet(t, router, "/slots/not-a-uuid")

		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", response.Code)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		mock := &mockBookingService{slotErr: errNoRows}

		router := newDiscoveryRouter(NewHandler(mock))

		response, body := doGet(t, router, "/slots/"+slotID.String())

		if response.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", response.Code)
		}

		if body["error"] == "" {
			t.Error("expected an error message")
		}
	})

	t.Run("success returns slot json", func(t *testing.T) {
		mock := &mockBookingService{
			slotResult: &AvailableSlot{
				ID:          slotID,
				ProviderID:  uuid.New(),
				DisplayName: "Dr. Test",
				Timezone:    "UTC",
				StartsAt:    time.Now().UTC().Add(time.Hour),
				EndsAt:      time.Now().UTC().Add(2 * time.Hour),
			},
		}

		router := newDiscoveryRouter(NewHandler(mock))

		response, body := doGet(t, router, "/slots/"+slotID.String())

		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", response.Code)
		}

		if body["id"] != slotID.String() {
			t.Errorf("expected slot id, got %v", body["id"])
		}

		if body["provider_name"] != "Dr. Test" {
			t.Errorf("expected provider_name, got %v", body["provider_name"])
		}

		// Sensitive provider fields must never appear.
		for _, forbidden := range []string{"email", "password_hash", "bio"} {
			if _, exists := body[forbidden]; exists {
				t.Errorf("response must not contain %q", forbidden)
			}
		}
	})
}
