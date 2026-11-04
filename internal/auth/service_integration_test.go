package auth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func newAuthTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// newTestService wires a real service, repository, and token manager.
func newTestService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()

	pool := newAuthTestPool(t)

	manager, err := NewTokenManager(testJWTSecret, 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	repo := NewRepository(pool)

	return NewService(repo, manager), pool
}

func uniqueEmail() string {
	return "auth-" + uuid.NewString() + "@example.com"
}

func deleteTestUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// refresh_tokens rows cascade on user deletion.
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
}

func TestRegisterCreatesUserAndStoresRefreshToken(t *testing.T) {
	service, pool := newTestService(t)

	ctx := context.Background()

	response, err := service.Register(ctx, uniqueEmail(), "correct horse battery")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, response.User.ID)

	if response.User.ID == uuid.Nil {
		t.Error("expected a user ID to be returned")
	}

	if response.User.Email != strings.ToLower(response.User.Email) {
		t.Error("expected stored email to be lowercase")
	}

	// The hash is populated in Go memory but must never cross the wire.
	wire, err := json.Marshal(response.User)
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}

	if strings.Contains(string(wire), "password_hash") {
		t.Error("password hash must not be serialized in the user JSON")
	}

	if response.TokenType != "Bearer" {
		t.Errorf("expected Bearer token type, got %q", response.TokenType)
	}

	if response.ExpiresIn != int64((15 * time.Minute).Seconds()) {
		t.Errorf("expected expires_in 900, got %d", response.ExpiresIn)
	}

	if _, err := service.tokens.ValidateAccessToken(response.AccessToken); err != nil {
		t.Errorf("returned access token should validate: %v", err)
	}

	// The refresh token must be stored as a hash, not as raw text.
	var storedHash []byte

	err = pool.QueryRow(ctx, `
		SELECT token_hash FROM refresh_tokens WHERE user_id = $1
	`, response.User.ID).Scan(&storedHash)
	if err != nil {
		t.Fatalf("load stored refresh token: %v", err)
	}

	if string(storedHash) != string(HashRefreshToken(response.RefreshToken)) {
		t.Error("stored hash does not match the returned refresh token")
	}

	var rawCount int

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM refresh_tokens
		WHERE user_id = $1 AND token_hash = $2
	`, response.User.ID, []byte(response.RefreshToken)).Scan(&rawCount)
	if err != nil {
		t.Fatalf("check raw token storage: %v", err)
	}

	if rawCount != 0 {
		t.Error("raw refresh token must never be stored in the database")
	}
}

func TestRegisterStoresBcryptHash(t *testing.T) {
	service, pool := newTestService(t)

	password := "correct horse battery"

	response, err := service.Register(context.Background(), uniqueEmail(), password)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, response.User.ID)

	var storedHash string

	err = pool.QueryRow(context.Background(), `
		SELECT password_hash FROM users WHERE id = $1
	`, response.User.ID).Scan(&storedHash)
	if err != nil {
		t.Fatalf("load stored password hash: %v", err)
	}

	if storedHash == password {
		t.Fatal("password must not be stored in plain text")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(storedHash),
		[]byte(password),
	); err != nil {
		t.Errorf("stored hash does not match the password: %v", err)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	service, pool := newTestService(t)

	email := uniqueEmail()

	first, err := service.Register(context.Background(), email, "correct horse battery")
	if err != nil {
		t.Fatalf("first register: %v", err)
	}

	deleteTestUser(t, pool, first.User.ID)

	if _, err := service.Register(
		context.Background(),
		email,
		"correct horse battery",
	); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	// The unique index is on LOWER(email).
	if _, err := service.Register(
		context.Background(),
		strings.ToUpper(email),
		"correct horse battery",
	); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected case-insensitive duplicate to fail, got %v", err)
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	service, _ := newTestService(t)

	if _, err := service.Register(
		context.Background(),
		uniqueEmail(),
		"short-pass",
	); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("expected ErrWeakPassword for 10-char password, got %v", err)
	}
}

func TestRegisterRejectsEmptyEmail(t *testing.T) {
	service, _ := newTestService(t)

	if _, err := service.Register(
		context.Background(),
		"   ",
		"correct horse battery",
	); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("expected ErrWeakPassword for empty email, got %v", err)
	}
}

func TestRegisterRejectsPasswordLongerThan72Bytes(t *testing.T) {
	service, _ := newTestService(t)

	// 40 runes of 'é' is 80 bytes but well below the 72-rune limit,
	// so only the byte-length check can reject it.
	longPassword := strings.Repeat("é", 40)

	if _, err := service.Register(
		context.Background(),
		uniqueEmail(),
		longPassword,
	); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}

func TestLoginSucceedsWithCaseInsensitiveEmail(t *testing.T) {
	service, pool := newTestService(t)

	email := uniqueEmail()

	registered, err := service.Register(
		context.Background(),
		email,
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, registered.User.ID)

	response, err := service.Login(
		context.Background(),
		"  "+strings.ToUpper(email)+"  ",
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("login with mixed-case email: %v", err)
	}

	if response.User.ID != registered.User.ID {
		t.Error("expected login to return the same user")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	service, pool := newTestService(t)

	registered, err := service.Register(
		context.Background(),
		uniqueEmail(),
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, registered.User.ID)

	if _, err := service.Login(
		context.Background(),
		registered.User.Email,
		"wrong horse battery",
	); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginRejectsUnknownEmail(t *testing.T) {
	service, _ := newTestService(t)

	if _, err := service.Login(
		context.Background(),
		"nobody-"+uuid.NewString()+"@example.com",
		"correct horse battery",
	); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginRejectsInactiveUser(t *testing.T) {
	service, pool := newTestService(t)

	registered, err := service.Register(
		context.Background(),
		uniqueEmail(),
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, registered.User.ID)

	_, err = pool.Exec(context.Background(), `
		UPDATE users SET is_active = FALSE WHERE id = $1
	`, registered.User.ID)
	if err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	if _, err := service.Login(
		context.Background(),
		registered.User.Email,
		"correct horse battery",
	); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for inactive user, got %v", err)
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	service, pool := newTestService(t)

	registered, err := service.Register(
		context.Background(),
		uniqueEmail(),
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, registered.User.ID)

	refreshed, err := service.Refresh(
		context.Background(),
		registered.RefreshToken,
	)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if refreshed.RefreshToken == registered.RefreshToken {
		t.Error("expected a new refresh token after rotation")
	}

	if refreshed.User.ID != registered.User.ID {
		t.Error("expected refresh to return the same user")
	}

	// The rotated token must work.
	if _, err := service.Refresh(
		context.Background(),
		refreshed.RefreshToken,
	); err != nil {
		t.Errorf("expected new refresh token to be usable, got %v", err)
	}

	// The old token must be revoked: reuse must fail.
	if _, err := service.Refresh(
		context.Background(),
		registered.RefreshToken,
	); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("expected reuse of rotated token to fail, got %v", err)
	}
}

func TestRefreshRejectsExpiredToken(t *testing.T) {
	service, pool := newTestService(t)

	registered, err := service.Register(
		context.Background(),
		uniqueEmail(),
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, registered.User.ID)

	// Age the stored token past its expiry.
	_, err = pool.Exec(context.Background(), `
		UPDATE refresh_tokens
		SET expires_at = NOW() - INTERVAL '1 hour'
		WHERE user_id = $1
	`, registered.User.ID)
	if err != nil {
		t.Fatalf("expire refresh token: %v", err)
	}

	if _, err := service.Refresh(
		context.Background(),
		registered.RefreshToken,
	); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("expected ErrInvalidRefresh for expired token, got %v", err)
	}
}

func TestRefreshRejectsTokenOfInactiveUser(t *testing.T) {
	service, pool := newTestService(t)

	registered, err := service.Register(
		context.Background(),
		uniqueEmail(),
		"correct horse battery",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	deleteTestUser(t, pool, registered.User.ID)

	_, err = pool.Exec(context.Background(), `
		UPDATE users SET is_active = FALSE WHERE id = $1
	`, registered.User.ID)
	if err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	if _, err := service.Refresh(
		context.Background(),
		registered.RefreshToken,
	); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("expected ErrInvalidRefresh for inactive user, got %v", err)
	}
}

func TestRefreshRejectsUnknownToken(t *testing.T) {
	service, _ := newTestService(t)

	raw, _, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	if _, err := service.Refresh(
		context.Background(),
		raw,
	); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("expected ErrInvalidRefresh for unknown token, got %v", err)
	}
}

func TestCreateUserMapsDuplicateEmail(t *testing.T) {
	pool := newAuthTestPool(t)

	repo := NewRepository(pool)

	email := uniqueEmail()

	first, err := repo.CreateUser(
		context.Background(),
		email,
		"irrelevant-hash",
	)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	deleteTestUser(t, pool, first.ID)

	if _, err := repo.CreateUser(
		context.Background(),
		email,
		"irrelevant-hash",
	); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestGetUserByEmailNotFound(t *testing.T) {
	pool := newAuthTestPool(t)

	repo := NewRepository(pool)

	_, err := repo.GetUserByEmail(
		context.Background(),
		"missing-"+uuid.NewString()+"@example.com",
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}
