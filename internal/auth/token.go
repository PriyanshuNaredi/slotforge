package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func NewTokenManager(
	secret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must contain at least 32 bytes")
	}

	if accessTTL <= 0 || refreshTTL <= 0 {
		return nil, errors.New("token lifetimes must be positive")
	}

	return &TokenManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

func (tm *TokenManager) GenerateAccessToken(
	userID uuid.UUID,
	role string,
) (string, error) {
	now := time.Now()

	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessTTL)),
			Issuer:    "slotforge",
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(tm.secret)
}

func (tm *TokenManager) ValidateAccessToken(
	tokenString string,
) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return tm.secret, nil
		},
		jwt.WithIssuer("slotforge"),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid access token")
	}

	return claims, nil
}

// GenerateRefreshToken returns the raw token and its SHA-256 hash.
// Only the hash should be stored in PostgreSQL.
func GenerateRefreshToken() (string, []byte, error) {
	raw := make([]byte, 32)

	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))

	return token, hash[:], nil
}

func HashRefreshToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func (tm *TokenManager) AccessTTL() time.Duration {
	return tm.accessTTL
}

func (tm *TokenManager) RefreshTTL() time.Duration {
	return tm.refreshTTL
}
