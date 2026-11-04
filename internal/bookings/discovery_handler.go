package bookings

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) SearchAvailableSlots(c *gin.Context) {
	input := SlotSearchInput{
		Limit:  20,
		Offset: 0,
	}

	if value := c.Query("provider_id"); value != "" {
		providerID, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider_id"})
			return
		}
		input.ProviderID = &providerID
	}

	if value := c.Query("from"); value != "" {
		from, err := time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be RFC3339"})
			return
		}
		input.From = &from
	}

	if value := c.Query("to"); value != "" {
		to, err := time.Parse(time.RFC3339, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be RFC3339"})
			return
		}
		input.To = &to
	}

	if input.From != nil && input.To != nil &&
		!input.From.Before(*input.To) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "from must be earlier than to",
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

	result, err := h.service.SearchAvailableSlotsWithCount(
		c.Request.Context(),
		input,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not search available slots",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
