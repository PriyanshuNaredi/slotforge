package providers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListProviderBookings(c *gin.Context) {
	providerID, err := getProviderUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	input := ProviderBookingListInput{
		Status: c.Query("status"),
		Limit:  20,
		Offset: 0,
	}

	if value := c.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid limit",
			})
			return
		}
		input.Limit = limit
	}

	if value := c.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid offset",
			})
			return
		}
		input.Offset = offset
	}

	if value := c.Query("from"); value != "" {
		from, err := time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "from must be RFC3339 format",
			})
			return
		}
		input.From = &from
	}

	if value := c.Query("to"); value != "" {
		to, err := time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "to must be RFC3339 format",
			})
			return
		}
		input.To = &to
	}

	result, err := h.service.ListProviderBookings(
		c.Request.Context(),
		providerID,
		input,
	)

	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "provider profile not found",
			})
			return
		}

		switch {
		case errors.Is(err, ErrInvalidBookingStatus),
			errors.Is(err, ErrInvalidDateRange),
			errors.Is(err, ErrInvalidLimit),
			errors.Is(err, ErrInvalidOffset):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not retrieve provider bookings",
			})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}
