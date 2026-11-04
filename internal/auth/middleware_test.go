package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// testJWTSecret is only used by tests. It is never a real credential.
const testJWTSecret = "unit-test-secret-0123456789-0123456789"

func newMiddlewareTestRouter(t *testing.T) (*gin.Engine, *TokenManager) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	manager, err := NewTokenManager(
		testJWTSecret,
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	router := gin.New()

	router.GET("/protected", AuthenticationMiddleware(manager), func(c *gin.Context) {
		userID, _ := c.Get(ContextUserID)
		role, _ := c.Get(ContextRole)

		c.JSON(http.StatusOK, gin.H{
			"user_id": userID,
			"role":    role,
		})
	})

	return router, manager
}

func TestAuthenticationMiddlewareAcceptsValidToken(t *testing.T) {
	router, manager := newMiddlewareTestRouter(t)

	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "customer")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.UserID != userID.String() {
		t.Errorf("expected user ID %s, got %q", userID, body.UserID)
	}

	if body.Role != "customer" {
		t.Errorf("expected role customer, got %q", body.Role)
	}
}

func TestAuthenticationMiddlewareRejectsMissingHeader(t *testing.T) {
	router, _ := newMiddlewareTestRouter(t)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestAuthenticationMiddlewareRejectsMalformedHeader(t *testing.T) {
	router, _ := newMiddlewareTestRouter(t)

	cases := map[string]string{
		"no bearer prefix": "SomeTokenValue",
		"wrong scheme":     "Basic SomeTokenValue",
		"empty token":      "Bearer ",
		"blank token":      "Bearer    ",
		"extra segments":   "Bearer a b",
	}

	for name, header := range cases {
		request := httptest.NewRequest(http.MethodGet, "/protected", nil)
		request.Header.Set("Authorization", header)

		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, response.Code)
		}
	}
}

func TestAuthenticationMiddlewareAcceptsLowercaseBearer(t *testing.T) {
	router, manager := newMiddlewareTestRouter(t)

	token, err := manager.GenerateAccessToken(uuid.New(), "customer")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "bearer "+token)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected lowercase bearer scheme to be accepted, got %d", response.Code)
	}
}

func TestAuthenticationMiddlewareRejectsGarbageToken(t *testing.T) {
	router, _ := newMiddlewareTestRouter(t)

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer invalid.token.value")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestAuthenticationMiddlewareRejectsExpiredToken(t *testing.T) {
	router, manager := newMiddlewareTestRouter(t)

	now := time.Now()

	token := signCustomToken(t, manager.secret, jwt.SigningMethodHS256, Claims{
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "slotforge",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", response.Code)
	}
}

func TestAuthenticationMiddlewareRejectsWrongSecretToken(t *testing.T) {
	router, _ := newMiddlewareTestRouter(t)

	otherManager, err := NewTokenManager(
		"other-secret-0123456789-0123456789-xx",
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	token, err := otherManager.GenerateAccessToken(uuid.New(), "customer")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for token signed with wrong secret, got %d", response.Code)
	}
}

func TestRequireRolesAllowsConfiguredRole(t *testing.T) {
	c, response := newRoleTestContext(t)

	c.Set(ContextRole, "provider")

	RequireRoles("provider")(c)

	if response.Code != http.StatusOK {
		t.Fatalf("expected provider role to pass, got %d", response.Code)
	}
}

func TestRequireRolesRejectsOtherRole(t *testing.T) {
	c, response := newRoleTestContext(t)

	c.Set(ContextRole, "customer")

	RequireRoles("provider")(c)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestRequireRolesRejectsMissingRole(t *testing.T) {
	c, response := newRoleTestContext(t)

	RequireRoles("provider")(c)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when role is missing, got %d", response.Code)
	}
}

func TestRequireRolesAcceptsAnyAllowedRole(t *testing.T) {
	for _, role := range []string{"provider", "admin"} {
		c, response := newRoleTestContext(t)

		c.Set(ContextRole, role)

		RequireRoles("provider", "admin")(c)

		if response.Code != http.StatusOK {
			t.Errorf("role %s: expected 200, got %d", role, response.Code)
		}
	}
}

func TestRequireRolesRejectsNonStringValue(t *testing.T) {
	c, response := newRoleTestContext(t)

	c.Set(ContextRole, 42)

	RequireRoles("provider")(c)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-string role, got %d", response.Code)
	}
}

func newRoleTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	response := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	return c, response
}
