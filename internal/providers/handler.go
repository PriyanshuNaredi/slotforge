package providers

import (
	"errors"
	"net/http"

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

// RegisterRoutes registers provider profile endpoints.
// Authentication and provider-role checks are applied in main.go.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/providers/profile", h.CreateProfile)
	router.GET("/providers/profile/me", h.GetMyProfile)

	router.POST("/providers/slots", h.CreateSlot)
	router.GET("/providers/slots", h.ListSlots)
	router.GET("/providers/slots/:id", h.GetSlot)
	router.PATCH("/providers/slots/:id/close", h.CloseSlot)

	router.GET("/providers/bookings", h.ListProviderBookings)
	router.PATCH("/providers/bookings/:id/status", h.UpdateProviderBookingStatus)
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

		if errors.Is(err, ErrSlotOverlap) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create slot"})
		return
	}

	c.JSON(http.StatusCreated, slot)
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
