package providers

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Bio         *string   `json:"bio,omitempty"`
	Timezone    string    `json:"timezone"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateProfileInput struct {
	DisplayName string  `json:"display_name" binding:"required,min=2,max=150"`
	Bio         *string `json:"bio,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
}

type ProviderBooking struct {
	ID            uuid.UUID `json:"id"`
	SlotID        uuid.UUID `json:"slot_id"`
	CustomerID    uuid.UUID `json:"customer_id"`
	CustomerEmail string    `json:"customer_email"`
	Status        string    `json:"status"`
	Notes         *string   `json:"notes,omitempty"`
	SlotStartsAt  time.Time `json:"slot_starts_at"`
	SlotEndsAt    time.Time `json:"slot_ends_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProviderBookingListInput struct {
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type ProviderBookingListResult struct {
	Bookings []ProviderBooking `json:"bookings"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
	Total    int64             `json:"total"`
}

// PublicProvider is the only shape the unauthenticated discovery endpoints
// may return. It intentionally excludes every users-table field: email,
// password hash, role, and account status never cross the wire here.
type PublicProvider struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Bio         *string   `json:"bio,omitempty"`
	Timezone    string    `json:"timezone"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProviderListInput struct {
	Search        string
	AvailableFrom *time.Time
	AvailableTo   *time.Time
	Limit         int
	Offset        int
}

type ProviderListResult struct {
	Items  []PublicProvider `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}
