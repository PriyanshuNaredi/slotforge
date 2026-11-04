package notifications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateWithDelivery creates an in-app notification and its email delivery
// record in the caller's transaction. Both succeed or both roll back.
func (r *Repository) CreateWithDelivery(
	ctx context.Context,
	tx pgx.Tx,
	input CreateInput,
) (*Notification, *Delivery, error) {
	notification, err := r.Create(ctx, tx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("create notification: %w", err)
	}

	var recipientEmail string

	err = tx.QueryRow(ctx, `
		SELECT email
		FROM users
		WHERE id = $1
	`, input.UserID).Scan(&recipientEmail)

	if err != nil {
		return nil, nil, fmt.Errorf("get notification recipient email: %w", err)
	}

	deliveryRepo := NewDeliveryRepository(r.db)

	delivery, err := deliveryRepo.Create(ctx, tx, CreateDeliveryInput{
		NotificationID: notification.ID,
		RecipientEmail: recipientEmail,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("create email delivery: %w", err)
	}

	return notification, delivery, nil
}
