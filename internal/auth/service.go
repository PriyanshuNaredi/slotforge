package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 12 characters")
	ErrPasswordTooLong    = errors.New("password must not exceed 72 bytes")
	ErrTooManyAttempts    = errors.New("too many login attempts; try again later")
)

// Login throttle: per-account fixed window. Keyed by email, not IP, so
// IP spoofing cannot bypass it. A successful login resets the counter.
const (
	loginAttemptLimit  = 10
	loginAttemptWindow = 5 * time.Minute
	// maxTrackedLoginWindows caps the failure map so an attacker cycling
	// distinct email addresses cannot grow process memory without bound.
	maxTrackedLoginWindows = 10_000
)

type Service struct {
	repo    *Repository
	tokens  *TokenManager
	fails   map[string]*loginWindow
	failsMu sync.Mutex
}

// loginWindow counts failed logins for one account within a fixed window.
type loginWindow struct {
	start time.Time
	count int
}

func NewService(repo *Repository, tokens *TokenManager) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		fails:  make(map[string]*loginWindow),
	}
}

// loginThrottled reports whether the account has exceeded the failed-login
// budget for the current window.
func (s *Service) loginThrottled(email string) bool {
	s.failsMu.Lock()
	defer s.failsMu.Unlock()

	w, ok := s.fails[email]
	if !ok {
		return false
	}
	if time.Since(w.start) >= loginAttemptWindow {
		delete(s.fails, email)
		return false
	}
	return w.count >= loginAttemptLimit
}

// recordLoginFailure increments the failed-login counter for the account.
func (s *Service) recordLoginFailure(email string) {
	s.failsMu.Lock()
	defer s.failsMu.Unlock()

	w, ok := s.fails[email]
	if !ok || time.Since(w.start) >= loginAttemptWindow {
		// Bound the map: an attacker spraying distinct addresses must not
		// grow it forever. Sweep expired entries; if still full, skip
		// tracking the new key (fail-open — the throttle is a mitigation,
		// not the auth control).
		if len(s.fails) >= maxTrackedLoginWindows {
			s.sweepExpiredLocked()
			if len(s.fails) >= maxTrackedLoginWindows {
				return
			}
		}
		s.fails[email] = &loginWindow{start: time.Now(), count: 1}
		return
	}
	w.count++
}

// sweepExpiredLocked drops windows whose fixed window has elapsed. Caller
// must hold failsMu.
func (s *Service) sweepExpiredLocked() {
	now := time.Now()
	for email, w := range s.fails {
		if now.Sub(w.start) >= loginAttemptWindow {
			delete(s.fails, email)
		}
	}
}

// recordLoginSuccess clears the counter after a successful login.
func (s *Service) recordLoginSuccess(email string) {
	s.failsMu.Lock()
	defer s.failsMu.Unlock()
	delete(s.fails, email)
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

	if s.loginThrottled(email) {
		return nil, ErrTooManyAttempts
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		s.recordLoginFailure(email)
		return nil, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		s.recordLoginFailure(email)
		return nil, ErrInvalidCredentials
	}

	s.recordLoginSuccess(email)

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
