package notifications

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/notifications", h.List)
	router.GET("/notifications/unread-count", h.CountUnread)
	router.PATCH("/notifications/:id/read", h.MarkAsRead)
	router.PATCH("/notifications/read-all", h.MarkAllAsRead)
}

func notificationUserID(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(auth.ContextUserID)
	if !exists {
		return uuid.Nil, errors.New("user identity missing")
	}

	idString, ok := value.(string)
	if !ok {
		return uuid.Nil, errors.New("invalid user identity")
	}

	return uuid.Parse(idString)
}

func (h *Handler) List(c *gin.Context) {
	userID, err := notificationUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	limit := 20
	offset := 0

	if value := c.Query("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
	}

	if value := c.Query("offset"); value != "" {
		offset, err = strconv.Atoi(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
			return
		}
	}

	result, err := h.service.List(
		c.Request.Context(),
		userID,
		ListInput{Limit: limit, Offset: offset},
	)

	if err != nil {
		if errors.Is(err, ErrInvalidPagination) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not retrieve notifications",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) CountUnread(c *gin.Context) {
	userID, err := notificationUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	count, err := h.service.CountUnread(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not count unread notifications",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"unread_count": count,
	})
}

func (h *Handler) MarkAsRead(c *gin.Context) {
	userID, err := notificationUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid notification ID",
		})
		return
	}

	notification, err := h.service.MarkAsRead(
		c.Request.Context(),
		userID,
		notificationID,
	)

	if err != nil {
		if errors.Is(err, ErrNotificationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "notification not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not mark notification as read",
		})
		return
	}

	c.JSON(http.StatusOK, notification)
}

func (h *Handler) MarkAllAsRead(c *gin.Context) {
	userID, err := notificationUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	updated, err := h.service.MarkAllAsRead(
		c.Request.Context(),
		userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not mark notifications as read",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"updated_count": updated,
	})
}
