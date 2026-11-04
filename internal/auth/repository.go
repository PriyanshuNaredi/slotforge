package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailTaken     = errors.New("email is already registered")
	ErrInvalidRefresh = errors.New("refresh token is invalid or expired")
	ErrInactiveUser   = errors.New("user account is inactive")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(
	ctx context.Context,
	email string,
	passwordHash string,
) (*User, error) {
	var user User

	err := r.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, 'customer')
		RETURNING id, email, password_hash, role, is_active, created_at
	`, email, passwordHash).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "users_email_lower_unique" {
			return nil, ErrEmailTaken
		}

		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

func (r *Repository) GetUserByEmail(
	ctx context.Context,
	email string,
) (*User, error) {
	var user User

	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, is_active, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return &user, nil
}

func (r *Repository) CreateRefreshToken(
	ctx context.Context,
	userID uuid.UUID,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)

	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}

	return nil
}

// RotateRefreshToken atomically revokes the old token and stores
// its replacement. Concurrent attempts to use the same token
// cannot both succeed.
func (r *Repository) RotateRefreshToken(
	ctx context.Context,
	oldHash []byte,
	newHash []byte,
	newExpiresAt time.Time,
) (*User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var userID uuid.UUID

	err = tx.QueryRow(ctx, `
		UPDATE refresh_tokens AS rt
		SET revoked_at = NOW()
		WHERE rt.token_hash = $1
		  AND rt.revoked_at IS NULL
		  AND rt.expires_at > NOW()
		  AND EXISTS (
		      SELECT 1
		      FROM users u
		      WHERE u.id = rt.user_id
		        AND u.is_active = TRUE
		  )
		RETURNING rt.user_id
	`, oldHash).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidRefresh
	}
	if err != nil {
		return nil, fmt.Errorf("revoke old refresh token: %w", err)
	}

	var user User

	err = tx.QueryRow(ctx, `
		SELECT id, email, password_hash, role, is_active, created_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("load refresh token user: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, user.ID, newHash, newExpiresAt)

	if err != nil {
		return nil, fmt.Errorf("store replacement refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit refresh transaction: %w", err)
	}

	return &user, nil
}
