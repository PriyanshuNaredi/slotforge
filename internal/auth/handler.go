package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/logging"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/auth/register", h.Register)
	router.POST("/auth/login", h.Login)
	router.POST("/auth/refresh", h.Refresh)
}

// RegisterProtectedRoutes adds endpoints that require authentication.
func (h *Handler) RegisterProtectedRoutes(group *gin.RouterGroup) {
	group.POST("/auth/logout", h.Logout)
}

func (h *Handler) Register(c *gin.Context) {
	var input RegisterInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "valid email and password are required",
		})
		return
	}

	response, err := h.service.Register(
		c.Request.Context(),
		input.Email,
		input.Password,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

		case errors.Is(err, ErrWeakPassword),
			errors.Is(err, ErrPasswordTooLong):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not register user",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *Handler) Login(c *gin.Context) {
	var input LoginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "valid email and password are required",
		})
		return
	}

	response, err := h.service.Login(
		c.Request.Context(),
		input.Email,
		input.Password,
	)

	if err != nil {
		if errors.Is(err, ErrTooManyAttempts) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many login attempts; try again later",
			})
			return
		}

		if errors.Is(err, ErrInvalidCredentials) {
			// Event log only: no email, password, or token is ever logged.
			logging.From(c).Warn("auth_login_failed")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid email or password",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not log in",
		})
		return
	}

	logging.From(c).Info("auth_login_succeeded",
		slog.String("user_id", response.User.ID.String()),
		slog.String("role", response.User.Role),
	)
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Refresh(c *gin.Context) {
	var input RefreshInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "refresh_token is required",
		})
		return
	}

	response, err := h.service.Refresh(
		c.Request.Context(),
		input.RefreshToken,
	)

	if err != nil {
		if errors.Is(err, ErrInvalidRefresh) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired refresh token",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not refresh session",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) Logout(c *gin.Context) {
	rawID, ok := c.Get(ContextUserID)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	strID, ok := rawID.(string)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid session",
		})
		return
	}

	userID, err := uuid.Parse(strID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid session",
		})
		return
	}

	var input LogoutInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "refresh_token is required",
		})
		return
	}

	// Idempotent by contract: unknown/expired/already-revoked tokens are
	// a successful no-op, so the client always ends up logged out.
	if err := h.service.Logout(c.Request.Context(), userID, input.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not log out",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
