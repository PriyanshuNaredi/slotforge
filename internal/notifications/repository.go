package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotificationNotFound = errors.New("notification not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a notification using the caller's transaction.
// The caller is responsible for committing or rolling back.
func (r *Repository) Create(
	ctx context.Context,
	tx pgx.Tx,
	input CreateInput,
) (*Notification, error) {
	var notification Notification

	err := tx.QueryRow(ctx, `
        INSERT INTO notifications (
            user_id,
            booking_id,
            type,
            title,
            message
        )
        VALUES ($1, $2, $3, $4, $5)
        RETURNING
            id,
            user_id,
            booking_id,
            type,
            title,
            message,
            read_at,
            delivery_status,
            attempts,
            available_at,
            sent_at,
            last_error,
            created_at,
            updated_at
    `,
		input.UserID,
		input.BookingID,
		input.Type,
		input.Title,
		input.Message,
	).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.BookingID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.ReadAt,
		&notification.DeliveryStatus,
		&notification.Attempts,
		&notification.AvailableAt,
		&notification.SentAt,
		&notification.LastError,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	return &notification, nil
}

// GetByID retrieves a notification belonging to the specified user.
func (r *Repository) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) (*Notification, error) {
	var notification Notification

	err := r.db.QueryRow(ctx, `
        SELECT
            id,
            user_id,
            booking_id,
            type,
            title,
            message,
            read_at,
            delivery_status,
            attempts,
            available_at,
            sent_at,
            last_error,
            created_at,
            updated_at
        FROM notifications
        WHERE id = $1 AND user_id = $2
    `, notificationID, userID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.BookingID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.ReadAt,
		&notification.DeliveryStatus,
		&notification.Attempts,
		&notification.AvailableAt,
		&notification.SentAt,
		&notification.LastError,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("get notification: %w", err)
	}

	return &notification, nil
}

// GetByIDForDelivery retrieves a notification by ID without user scoping.
// It is used by the email delivery worker, which only has the delivery's
// notification ID.
func (r *Repository) GetByIDForDelivery(
	ctx context.Context,
	notificationID uuid.UUID,
) (*Notification, error) {
	var notification Notification

	err := r.db.QueryRow(ctx, `
        SELECT
            id,
            user_id,
            booking_id,
            type,
            title,
            message,
            read_at,
            delivery_status,
            attempts,
            available_at,
            sent_at,
            last_error,
            created_at,
            updated_at
        FROM notifications
        WHERE id = $1
    `, notificationID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.BookingID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.ReadAt,
		&notification.DeliveryStatus,
		&notification.Attempts,
		&notification.AvailableAt,
		&notification.SentAt,
		&notification.LastError,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("get notification for delivery: %w", err)
	}

	return &notification, nil
}

// ListByUser retrieves a paginated list of notifications for a user.
func (r *Repository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	input ListInput,
) (*NotificationListResult, error) {
	var total int64

	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1
	`, userID).Scan(&total)

	if err != nil {
		return nil, fmt.Errorf("count user notifications: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			user_id,
			booking_id,
			type,
			title,
			message,
			read_at,
			delivery_status,
			attempts,
			available_at,
			sent_at,
			last_error,
			created_at,
			updated_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, input.Limit, input.Offset)

	if err != nil {
		return nil, fmt.Errorf("list user notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]Notification, 0)

	for rows.Next() {
		var notification Notification

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.BookingID,
			&notification.Type,
			&notification.Title,
			&notification.Message,
			&notification.ReadAt,
			&notification.DeliveryStatus,
			&notification.Attempts,
			&notification.AvailableAt,
			&notification.SentAt,
			&notification.LastError,
			&notification.CreatedAt,
			&notification.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}

	return &NotificationListResult{
		Notifications: notifications,
		Limit:         input.Limit,
		Offset:        input.Offset,
		Total:         total,
	}, nil
}

// CountUnread returns the number of unread notifications for a user.
func (r *Repository) CountUnread(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	var count int64

	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1
		  AND read_at IS NULL
	`, userID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}

	return count, nil
}

// MarkAsRead marks one notification as read.
// The user_id condition prevents users from modifying another user's notification.
func (r *Repository) MarkAsRead(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) (*Notification, error) {
	var notification Notification

	err := r.db.QueryRow(ctx, `
		UPDATE notifications
		SET
			read_at = COALESCE(read_at, NOW()),
			updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2
		RETURNING
			id,
			user_id,
			booking_id,
			type,
			title,
			message,
			read_at,
			delivery_status,
			attempts,
			available_at,
			sent_at,
			last_error,
			created_at,
			updated_at
	`, notificationID, userID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.BookingID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.ReadAt,
		&notification.DeliveryStatus,
		&notification.Attempts,
		&notification.AvailableAt,
		&notification.SentAt,
		&notification.LastError,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("mark notification as read: %w", err)
	}

	return &notification, nil
}

// MarkAllAsRead marks all unread notifications belonging to a user as read.
func (r *Repository) MarkAllAsRead(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET
			read_at = NOW(),
			updated_at = NOW()
		WHERE user_id = $1
		  AND read_at IS NULL
	`, userID)

	if err != nil {
		return 0, fmt.Errorf("mark all notifications as read: %w", err)
	}

	return result.RowsAffected(), nil
}
