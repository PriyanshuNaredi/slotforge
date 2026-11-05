package bookings

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) RescheduleBooking(c *gin.Context) {
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

	var input RescheduleInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "slot_id is required",
		})
		return
	}

	booking, err := h.service.RescheduleBooking(
		c.Request.Context(),
		bookingID,
		customerID,
		input.SlotID,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})

		case errors.Is(err, ErrBookingNotReschedulable):
			c.JSON(http.StatusConflict, gin.H{
				"error": "booking cannot be rescheduled",
			})

		case errors.Is(err, ErrSlotNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "slot not found",
			})

		case errors.Is(err, ErrSlotClosed):
			c.JSON(http.StatusConflict, gin.H{
				"error": "availability slot is closed",
			})

		case errors.Is(err, ErrSlotBooked):
			c.JSON(http.StatusConflict, gin.H{
				"error": "availability slot is already booked",
			})

		case errors.Is(err, ErrSlotCrossProvider):
			c.JSON(http.StatusConflict, gin.H{
				"error": "target slot belongs to a different provider",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not reschedule booking",
			})
		}
		return
	}

	c.JSON(http.StatusOK, booking)
}
