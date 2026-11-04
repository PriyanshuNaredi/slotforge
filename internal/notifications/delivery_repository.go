package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidDeliveryInput = errors.New("invalid delivery input")
	ErrDeliveryNotFound     = errors.New("delivery not found")
)

type DeliveryRepository struct {
	db *pgxpool.Pool
}

func NewDeliveryRepository(db *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

func (r *DeliveryRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	input CreateDeliveryInput,
) (*Delivery, error) {
	if input.NotificationID == uuid.Nil ||
		!strings.Contains(input.RecipientEmail, "@") {
		return nil, ErrInvalidDeliveryInput
	}

	var delivery Delivery

	err := tx.QueryRow(ctx, `
		INSERT INTO notification_deliveries (
			notification_id,
			recipient_email
		)
		VALUES ($1, $2)
		RETURNING
			id,
			notification_id,
			recipient_email,
			status,
			attempts,
			available_at,
			locked_at,
			sent_at,
			last_error,
			created_at,
			updated_at
	`,
		input.NotificationID,
		input.RecipientEmail,
	).Scan(
		&delivery.ID,
		&delivery.NotificationID,
		&delivery.RecipientEmail,
		&delivery.Status,
		&delivery.Attempts,
		&delivery.AvailableAt,
		&delivery.LockedAt,
		&delivery.SentAt,
		&delivery.LastError,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create notification delivery: %w", err)
	}

	return &delivery, nil
}

// ClaimBatch atomically claims pending deliveries.
// SKIP LOCKED allows multiple workers to process different jobs concurrently.
func (r *DeliveryRepository) ClaimBatch(
	ctx context.Context,
	batchSize int,
) ([]Delivery, error) {
	if batchSize < 1 || batchSize > 100 {
		return nil, ErrInvalidDeliveryInput
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM notification_deliveries
			WHERE status = 'pending'
			  AND available_at <= NOW()
			ORDER BY available_at, created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE notification_deliveries AS d
		SET
			status = 'processing',
			locked_at = NOW(),
			attempts = attempts + 1,
			updated_at = NOW()
		FROM candidates
		WHERE d.id = candidates.id
		RETURNING
			d.id,
			d.notification_id,
			d.recipient_email,
			d.status,
			d.attempts,
			d.available_at,
			d.locked_at,
			d.sent_at,
			d.last_error,
			d.created_at,
			d.updated_at
	`, batchSize)

	if err != nil {
		return nil, fmt.Errorf("claim notification deliveries: %w", err)
	}

	deliveries := make([]Delivery, 0)

	for rows.Next() {
		var delivery Delivery

		if err := rows.Scan(
			&delivery.ID,
			&delivery.NotificationID,
			&delivery.RecipientEmail,
			&delivery.Status,
			&delivery.Attempts,
			&delivery.AvailableAt,
			&delivery.LockedAt,
			&delivery.SentAt,
			&delivery.LastError,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan claimed delivery: %w", err)
		}

		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate claimed deliveries: %w", err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delivery claim: %w", err)
	}

	return deliveries, nil
}

func (r *DeliveryRepository) MarkSent(
	ctx context.Context,
	deliveryID uuid.UUID,
) error {
	commandTag, err := r.db.Exec(ctx, `
		UPDATE notification_deliveries
		SET
			status = 'sent',
			sent_at = NOW(),
			locked_at = NULL,
			last_error = NULL,
			updated_at = NOW()
		WHERE id = $1
		  AND status = 'processing'
	`, deliveryID)

	if err != nil {
		return fmt.Errorf("mark delivery sent: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// ScheduleRetry schedules another attempt using exponential backoff.
// The attempt count is incremented when ClaimBatch claims the job.
func (r *DeliveryRepository) ScheduleRetry(
	ctx context.Context,
	deliveryID uuid.UUID,
	deliveryError string,
) error {
	commandTag, err := r.db.Exec(ctx, `
		UPDATE notification_deliveries
		SET
			status = CASE
				WHEN attempts >= $2 THEN 'failed'
				ELSE 'pending'
			END,
			available_at = CASE
				WHEN attempts >= $2 THEN available_at
				ELSE NOW() + make_interval(
					secs => LEAST(
						POWER(2, attempts)::INTEGER * 30,
						3600
					)
				)
			END,
			locked_at = NULL,
			last_error = $3,
			updated_at = NOW()
		WHERE id = $1
		  AND status = 'processing'
	`, deliveryID, MaxDeliveryAttempts, deliveryError)

	if err != nil {
		return fmt.Errorf("schedule delivery retry: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// RecoverStaleClaims returns jobs abandoned by crashed workers to the queue.
func (r *DeliveryRepository) RecoverStaleClaims(
	ctx context.Context,
	staleAfter time.Duration,
) (int64, error) {
	if staleAfter <= 0 {
		return 0, ErrInvalidDeliveryInput
	}

	commandTag, err := r.db.Exec(ctx, `
		UPDATE notification_deliveries
		SET
			status = CASE
				WHEN attempts >= $2 THEN 'failed'
				ELSE 'pending'
			END,
			available_at = NOW(),
			locked_at = NULL,
			last_error = COALESCE(last_error || '; ', '') ||
				'worker claim expired',
			updated_at = NOW()
		WHERE status = 'processing'
		  AND locked_at < NOW() - ($1 * INTERVAL '1 second')
	`, staleAfter.Seconds(), MaxDeliveryAttempts)

	if err != nil {
		return 0, fmt.Errorf("recover stale delivery claims: %w", err)
	}

	return commandTag.RowsAffected(), nil
}
