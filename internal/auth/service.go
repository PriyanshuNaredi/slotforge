package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 12 characters")
	ErrPasswordTooLong    = errors.New("password must not exceed 72 bytes")
)

type Service struct {
	repo   *Repository
	tokens *TokenManager
}

func NewService(repo *Repository, tokens *TokenManager) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
	}
}

func (s *Service) Register(
	ctx context.Context,
	email string,
	password string,
) (*AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	if email == "" || len(password) < 12 {
		return nil, ErrWeakPassword
	}

	// bcrypt only supports passwords up to 72 bytes.
	if len([]byte(password)) > 72 {
		return nil, ErrPasswordTooLong
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.CreateUser(ctx, email, string(passwordHash))
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, user)
}

func (s *Service) Login(
	ctx context.Context,
	email string,
	password string,
) (*AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, user)
}

func (s *Service) Refresh(
	ctx context.Context,
	rawRefreshToken string,
) (*AuthResponse, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" || len(rawRefreshToken) > 256 {
		return nil, ErrInvalidRefresh
	}

	newRawToken, newHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	user, err := s.repo.RotateRefreshToken(
		ctx,
		HashRefreshToken(rawRefreshToken),
		newHash,
		time.Now().Add(s.tokens.RefreshTTL()),
	)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.tokens.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRawToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokens.AccessTTL().Seconds()),
		User:         *user,
	}, nil
}

func (s *Service) issueTokens(
	ctx context.Context,
	user *User,
) (*AuthResponse, error) {
	accessToken, err := s.tokens.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	rawRefreshToken, refreshHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.tokens.RefreshTTL())

	err = s.repo.CreateRefreshToken(
		ctx,
		user.ID,
		refreshHash,
		expiresAt,
	)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokens.AccessTTL().Seconds()),
		User:         *user,
	}, nil
}

// Logout revokes the caller's own refresh token. Unknown, expired,
// already-revoked, and other users' tokens are all a successful no-op:
// logging out must be idempotent and must not leak token existence.
func (s *Service) Logout(
	ctx context.Context,
	userID uuid.UUID,
	rawRefreshToken string,
) error {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" || len(rawRefreshToken) > 256 {
		return nil
	}

	_, err := s.repo.RevokeRefreshToken(
		ctx,
		userID,
		HashRefreshToken(rawRefreshToken),
	)

	return err
}

// Useful for account-management operations in a later phase.
func (s *Service) ValidateUserID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}
