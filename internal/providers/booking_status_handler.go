package providers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UpdateBookingStatusInput struct {
	Status string `json:"status" binding:"required"`
}

func (h *Handler) UpdateProviderBookingStatus(c *gin.Context) {
	// 1. Get authenticated provider ID.
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	// 2. Parse booking ID.
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid booking ID",
		})
		return
	}

	// 3. Parse request body.
	var input UpdateBookingStatusInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "status is required",
		})
		return
	}

	// 4. Update booking status.
	booking, err := h.service.UpdateProviderBookingStatus(
		c.Request.Context(),
		providerID,
		bookingID,
		input.Status,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrProviderBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})

		case errors.Is(err, ErrInvalidBookingStatus):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid booking status",
			})

		case errors.Is(err, ErrInvalidBookingTransition):
			c.JSON(http.StatusConflict, gin.H{
				"error": "booking cannot transition to the requested status",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not update booking status",
			})
		}
		return
	}

	// 5. Return updated booking.
	c.JSON(http.StatusOK, booking)
}
