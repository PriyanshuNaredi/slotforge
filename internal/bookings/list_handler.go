package bookings

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListCustomerBookings(c *gin.Context) {
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

	input := BookingListInput{
		Limit:  20,
		Offset: 0,
	}

	if value := c.Query("status"); value != "" {
		status := BookingStatus(value)

		switch status {
		case StatusPending, StatusConfirmed, StatusCancelled,
			StatusCompleted, StatusNoShow:
			input.Status = &status
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid booking status",
			})
			return
		}
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

	result, err := h.service.ListCustomerBookings(
		c.Request.Context(),
		customerID,
		input,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve bookings",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
