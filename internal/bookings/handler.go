package bookings

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/priyanshunaredi/slotforge/internal/auth"
	"github.com/priyanshunaredi/slotforge/internal/logging"
)

type BookingService interface {
	CreateBooking(ctx context.Context, customerID uuid.UUID, input CreateBookingInput) (*Booking, error)
	GetBooking(ctx context.Context, id, callerID uuid.UUID, isAdmin bool) (*BookingDetail, error)
	SearchAvailableSlotsWithCount(ctx context.Context, input SlotSearchInput) (*SlotSearchResult, error)
	GetAvailableSlot(ctx context.Context, slotID uuid.UUID) (*AvailableSlot, error)
	ListCustomerBookings(ctx context.Context, customerID uuid.UUID, input BookingListInput) (*BookingListResult, error)
	CancelBooking(
		ctx context.Context,
		bookingID uuid.UUID,
		customerID uuid.UUID,
	) (*Booking, error)
	RescheduleBooking(
		ctx context.Context,
		bookingID uuid.UUID,
		customerID uuid.UUID,
		newSlotID uuid.UUID,
	) (*Booking, error)
}

type Handler struct {
	service BookingService
}

func NewHandler(service BookingService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/bookings", auth.RequireRoles("customer"), h.CreateBooking)
	router.GET("/bookings", auth.RequireRoles("customer"), h.ListCustomerBookings)
	router.GET("/bookings/:id", h.GetBooking)
	router.PATCH(
		"/bookings/:id/cancel",
		auth.RequireRoles("customer"),
		h.CancelBooking,
	)
	router.PATCH(
		"/bookings/:id/reschedule",
		auth.RequireRoles("customer"),
		h.RescheduleBooking,
	)
}

func (h *Handler) RegisterPublicRoutes(router *gin.RouterGroup) {
	router.GET("/slots", h.SearchAvailableSlots)
	router.GET("/slots/:id", h.GetAvailableSlot)
}

func getAuthenticatedUser(c *gin.Context) (uuid.UUID, string, error) {
	userIDValue, exists := c.Get(auth.ContextUserID)
	if !exists {
		return uuid.Nil, "", errors.New("user identity missing")
	}

	userIDString, ok := userIDValue.(string)
	if !ok {
		return uuid.Nil, "", errors.New("invalid user identity")
	}

	userID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil, "", err
	}

	roleValue, exists := c.Get(auth.ContextRole)
	if !exists {
		return uuid.Nil, "", errors.New("user role missing")
	}

	role, ok := roleValue.(string)
	if !ok {
		return uuid.Nil, "", errors.New("invalid user role")
	}

	return userID, role, nil
}

func (h *Handler) CreateBooking(c *gin.Context) {
	customerID, _, err := getAuthenticatedUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	var input CreateBookingInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	booking, err := h.service.CreateBooking(
		c.Request.Context(),
		customerID,
		input,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrSlotNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

		case errors.Is(err, ErrSlotClosed),
			errors.Is(err, ErrSlotBooked),
			errors.Is(err, ErrSlotBlocked):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not create booking",
			})
		}
		return
	}

	logging.From(c).Info("booking_created",
		slog.String("booking_id", booking.ID.String()),
		slog.String("slot_id", booking.SlotID.String()),
		slog.String("customer_id", booking.CustomerID.String()),
	)
	c.JSON(http.StatusCreated, booking)
}

func (h *Handler) GetBooking(c *gin.Context) {
	callerID, role, err := getAuthenticatedUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid booking ID",
		})
		return
	}

	booking, err := h.service.GetBooking(
		c.Request.Context(),
		id,
		callerID,
		role == "admin",
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve booking",
		})
		return
	}

	c.JSON(http.StatusOK, booking)
}
