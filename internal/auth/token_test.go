package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestAccessTokenGenerationAndValidation(t *testing.T) {
	manager, err := NewTokenManager(
		"01234567890123456789012345678901",
		15*time.Minute,
		7*24*time.Hour,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "customer")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	claims, err := manager.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf(
			"expected subject %s, got %s",
			userID,
			claims.Subject,
		)
	}

	if claims.Role != "customer" {
		t.Errorf(
			"expected role customer, got %s",
			claims.Role,
		)
	}
}

func TestTokenManagerRejectsShortSecret(t *testing.T) {
	_, err := NewTokenManager(
		"too-short",
		time.Minute,
		time.Hour,
	)

	if err == nil {
		t.Fatal("expected short JWT secret to be rejected")
	}
}

func TestAccessTokenRejectsInvalidToken(t *testing.T) {
	manager, err := NewTokenManager(
		"01234567890123456789012345678901",
		15*time.Minute,
		7*24*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = manager.ValidateAccessToken("invalid.token.value")

	if err == nil {
		t.Fatal("expected invalid token to be rejected")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	token1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}

	token2, hash2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}

	if token1 == token2 {
		t.Fatal("expected unique refresh tokens")
	}

	if string(hash1) == string(hash2) {
		t.Fatal("expected unique refresh token hashes")
	}

	if string(hash1) != string(HashRefreshToken(token1)) {
		t.Fatal("refresh token hash does not match")
	}
}

// signCustomToken builds a token outside GenerateAccessToken so tests
// can control individual claims, the algorithm, and the secret.
func signCustomToken(
	t *testing.T,
	secret []byte,
	method jwt.SigningMethod,
	claims Claims,
) string {
	t.Helper()

	token := jwt.NewWithClaims(method, claims)

	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign custom token: %v", err)
	}

	return signed
}

func newTestTokenManager(
	t *testing.T,
) *TokenManager {
	t.Helper()

	manager, err := NewTokenManager(
		testJWTSecret,
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	return manager
}

func TestTokenManagerRejectsNonPositiveLifetimes(t *testing.T) {
	if _, err := NewTokenManager(testJWTSecret, 0, time.Hour); err == nil {
		t.Error("expected zero access TTL to be rejected")
	}

	if _, err := NewTokenManager(testJWTSecret, time.Minute, -time.Hour); err == nil {
		t.Error("expected negative refresh TTL to be rejected")
	}
}

func TestAccessTokenRejectsExpiredToken(t *testing.T) {
	manager := newTestTokenManager(t)

	token := signCustomToken(t, manager.secret, jwt.SigningMethodHS256, Claims{
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "slotforge",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})

	if _, err := manager.ValidateAccessToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestAccessTokenRejectsWrongSecret(t *testing.T) {
	manager := newTestTokenManager(t)

	otherManager, err := NewTokenManager(
		"different-secret-0123456789abcdef0123",
		15*time.Minute,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create second token manager: %v", err)
	}

	token, err := otherManager.GenerateAccessToken(uuid.New(), "customer")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if _, err := manager.ValidateAccessToken(token); err == nil {
		t.Fatal("expected token signed with a different secret to be rejected")
	}
}

func TestAccessTokenRejectsWrongSigningAlgorithm(t *testing.T) {
	manager := newTestTokenManager(t)

	now := time.Now()

	token := signCustomToken(t, manager.secret, jwt.SigningMethodHS512, Claims{
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "slotforge",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})

	if _, err := manager.ValidateAccessToken(token); err == nil {
		t.Fatal("expected HS512 token to be rejected")
	}
}

func TestAccessTokenRequiresIssuer(t *testing.T) {
	manager := newTestTokenManager(t)

	now := time.Now()

	token := signCustomToken(t, manager.secret, jwt.SigningMethodHS256, Claims{
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})

	if _, err := manager.ValidateAccessToken(token); err == nil {
		t.Fatal("expected token with missing issuer to be rejected")
	}
}

func TestAccessTokenRequiresExpiration(t *testing.T) {
	manager := newTestTokenManager(t)

	token := signCustomToken(t, manager.secret, jwt.SigningMethodHS256, Claims{
		Role: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  uuid.NewString(),
			Issuer:   "slotforge",
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	})

	if _, err := manager.ValidateAccessToken(token); err == nil {
		t.Fatal("expected token without expiration to be rejected")
	}
}
