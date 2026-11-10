package providers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/priyanshunaredi/slotforge/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterPublicRoutes registers unauthenticated provider discovery
// endpoints. No authentication or role checks apply here; responses carry
// public profile fields only.
func (h *Handler) RegisterPublicRoutes(router *gin.RouterGroup) {
	router.GET("/providers", h.ListPublicProviders)
	router.GET("/providers/:id", h.GetPublicProvider)
}

// RegisterRoutes registers provider profile endpoints.
// Authentication and provider-role checks are applied in main.go.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/providers/profile", h.CreateProfile)
	router.GET("/providers/profile/me", h.GetMyProfile)

	router.POST("/providers/slots", h.CreateSlot)
	router.POST("/providers/slots/recurring", h.CreateRecurringSlots)
	router.GET("/providers/slots", h.ListSlots)
	router.GET("/providers/slots/:id", h.GetSlot)
	router.PATCH("/providers/slots/:id/close", h.CloseSlot)

	router.GET("/providers/bookings", h.ListProviderBookings)
	router.PATCH("/providers/bookings/:id/status", h.UpdateProviderBookingStatus)

	router.POST("/providers/unavailability", h.CreateUnavailablePeriod)
	router.GET("/providers/unavailability", h.ListUnavailablePeriods)
	router.DELETE("/providers/unavailability/:id", h.DeleteUnavailablePeriod)
}

// Extract the authenticated user's ID from Gin context.
func getProviderUserID(c *gin.Context) (uuid.UUID, error) {
	userIDValue, exists := c.Get(auth.ContextUserID)
	if !exists {
		return uuid.Nil, errors.New("user identity missing")
	}

	userIDString, ok := userIDValue.(string)
	if !ok {
		return uuid.Nil, errors.New("invalid user identity")
	}

	userID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil, errors.New("invalid user ID")
	}

	return userID, nil
}

// GET /api/v1/providers — public provider directory.
func (h *Handler) ListPublicProviders(c *gin.Context) {
	input := ProviderListInput{
		Limit:  20,
		Offset: 0,
	}

	if value := strings.TrimSpace(c.Query("search")); value != "" {
		input.Search = value
	}

	if value := c.Query("available_from"); value != "" {
		from, err := time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "available_from must be RFC3339",
			})
			return
		}
		input.AvailableFrom = &from
	}

	if value := c.Query("available_to"); value != "" {
		to, err := time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "available_to must be RFC3339",
			})
			return
		}
		input.AvailableTo = &to
	}

	if (input.AvailableFrom == nil) != (input.AvailableTo == nil) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "available_from and available_to must be provided together",
		})
		return
	}

	if input.AvailableFrom != nil &&
		!input.AvailableTo.After(*input.AvailableFrom) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "available_to must be after available_from",
		})
		return
	}

	if value := c.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "limit must be between 1 and 100",
			})
			return
		}
		input.Limit = limit
	}

	if value := c.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "offset must be zero or greater",
			})
			return
		}
		input.Offset = offset
	}

	result, err := h.service.ListPublicProviders(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not list providers",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GET /api/v1/providers/:id — public provider detail. Missing profiles
// and unknown IDs both return 404 without revealing account existence.
func (h *Handler) GetPublicProvider(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid provider ID",
		})
		return
	}

	provider, err := h.service.GetPublicProvider(c.Request.Context(), providerID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "provider not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve provider",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// POST /api/v1/providers/profile
func (h *Handler) CreateProfile(c *gin.Context) {
	userID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	var input CreateProfileInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid profile details",
		})
		return
	}

	profile, err := h.service.CreateProfile(
		c.Request.Context(),
		userID,
		input,
	)

	if err != nil {
		if errors.Is(err, ErrProfileExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, profile)
}

// GET /api/v1/providers/profile/me
func (h *Handler) GetMyProfile(c *gin.Context) {
	userID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	profile, err := h.service.GetMyProfile(
		c.Request.Context(),
		userID,
	)

	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "provider profile not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve provider profile",
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (h *Handler) CreateSlot(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input CreateSlotInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if input.StartsAt.IsZero() || input.EndsAt.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start and end times are required"})
		return
	}

	slot, err := h.service.CreateSlot(c.Request.Context(), providerID, input)
	if err != nil {
		if errors.Is(err, ErrInvalidSlotTime) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if errors.Is(err, ErrSlotOverlap) || errors.Is(err, ErrSlotBlockedByUnavailability) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create slot"})
		return
	}

	c.JSON(http.StatusCreated, slot)
}

// CreateRecurringSlots materializes weekly recurring availability.
func (h *Handler) CreateRecurringSlots(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input RecurringRuleInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recurrence rule"})
		return
	}

	result, err := h.service.CreateRecurringSlots(c.Request.Context(), providerID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidTimezone),
			errors.Is(err, ErrInvalidRuleWindow),
			errors.Is(err, ErrInvalidRuleTime),
			errors.Is(err, ErrInvalidDaysOfWeek),
			errors.Is(err, ErrRuleTooManySlots),
			errors.Is(err, ErrRuleInPast),
			errors.Is(err, ErrRuleNoMatchingDays):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		case errors.Is(err, ErrSlotOverlap):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create recurring slots"})
		}
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (h *Handler) CreateUnavailablePeriod(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input CreateUnavailablePeriodInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if input.StartsAt.IsZero() || input.EndsAt.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start and end times are required"})
		return
	}

	period, err := h.service.CreateUnavailablePeriod(c.Request.Context(), providerID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidUnavailability):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnavailableHasBookings), errors.Is(err, ErrUnavailableLocked):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create unavailable period"})
		}
		return
	}

	c.JSON(http.StatusCreated, period)
}

func (h *Handler) ListUnavailablePeriods(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	periods, err := h.service.ListUnavailablePeriods(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve unavailable periods"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unavailable_periods": periods})
}

func (h *Handler) DeleteUnavailablePeriod(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	periodID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid unavailable period ID"})
		return
	}

	err = h.service.DeleteUnavailablePeriod(c.Request.Context(), providerID, periodID)
	if err != nil {
		if errors.Is(err, ErrUnavailableNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete unavailable period"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ListSlots(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	slots, err := h.service.ListSlots(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve slots"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"slots": slots})
}

func (h *Handler) GetSlot(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	slotID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slot ID"})
		return
	}

	slot, err := h.service.GetSlot(c.Request.Context(), providerID, slotID)
	if err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "slot not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve slot"})
		return
	}

	c.JSON(http.StatusOK, slot)
}

func (h *Handler) CloseSlot(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	slotID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slot ID"})
		return
	}

	err = h.service.CloseSlot(c.Request.Context(), providerID, slotID)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlotNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "slot not found"})
		case errors.Is(err, ErrSlotHasActiveBooking):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not close slot"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "slot closed successfully"})
}
