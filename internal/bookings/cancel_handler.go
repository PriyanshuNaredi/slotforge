package bookings

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/logging"
)

func (h *Handler) CancelBooking(c *gin.Context) {
	customerID, role, err := getAuthenticatedUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	if role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "customer access required",
		})
		return
	}

	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid booking ID",
		})
		return
	}

	booking, err := h.service.CancelBooking(
		c.Request.Context(),
		bookingID,
		customerID,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})

		case errors.Is(err, ErrBookingNotCancellable):
			c.JSON(http.StatusConflict, gin.H{
				"error": "booking cannot be cancelled",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not cancel booking",
			})
		}
		return
	}

	logging.From(c).Info("booking_cancelled",
		slog.String("booking_id", booking.ID.String()),
		slog.String("customer_id", booking.CustomerID.String()),
	)
	c.JSON(http.StatusOK, booking)
}
