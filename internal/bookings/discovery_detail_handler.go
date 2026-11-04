package bookings

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) GetAvailableSlot(c *gin.Context) {
	slotID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid slot ID",
		})
		return
	}

	slot, err := h.service.GetAvailableSlot(
		c.Request.Context(),
		slotID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "available slot not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve available slot",
		})
		return
	}

	c.JSON(http.StatusOK, slot)
}
