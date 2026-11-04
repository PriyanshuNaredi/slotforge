package notifications

import (
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliverySent       DeliveryStatus = "sent"
	DeliveryFailed     DeliveryStatus = "failed"
)

type Delivery struct {
	ID             uuid.UUID      `json:"id"`
	NotificationID uuid.UUID      `json:"notification_id"`
	RecipientEmail string         `json:"recipient_email"`
	Status         DeliveryStatus `json:"status"`
	Attempts       int            `json:"attempts"`
	AvailableAt    time.Time      `json:"available_at"`
	LockedAt       *time.Time     `json:"locked_at,omitempty"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	LastError      *string        `json:"last_error,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateDeliveryInput struct {
	NotificationID uuid.UUID
	RecipientEmail string
}

type ClaimDeliveriesInput struct {
	BatchSize int
}

const (
	DefaultDeliveryBatchSize = 10
	MaxDeliveryAttempts      = 5
)
