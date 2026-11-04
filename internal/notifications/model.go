package notifications

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID             uuid.UUID      `json:"id"`
	UserID         uuid.UUID      `json:"user_id"`
	BookingID      *uuid.UUID     `json:"booking_id,omitempty"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Message        string         `json:"message"`
	ReadAt         *time.Time     `json:"read_at,omitempty"`
	DeliveryStatus DeliveryStatus `json:"delivery_status"`
	Attempts       int            `json:"attempts"`
	AvailableAt    time.Time      `json:"available_at"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	LastError      *string        `json:"last_error,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateInput struct {
	UserID    uuid.UUID
	BookingID *uuid.UUID
	Type      string
	Title     string
	Message   string
}

type ListInput struct {
	Limit  int
	Offset int
}

type NotificationListResult struct {
	Notifications []Notification `json:"notifications"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
	Total         int64          `json:"total"`
}
