package providers

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidUnavailability = errors.New("invalid unavailable period: end must be after start")
	// ErrUnavailableHasBookings is returned when a provider tries to block a
	// window that overlaps active bookings. Existing confirmed bookings are
	// never silently cancelled — the provider must cancel them first.
	ErrUnavailableHasBookings = errors.New("period overlaps active bookings; cancel them first")
	ErrUnavailableNotFound    = errors.New("unavailable period not found")
	// ErrUnavailableLocked is transient: the transaction deadlocked or lost a
	// serialization race with a concurrent update. Retrying is safe.
	ErrUnavailableLocked = errors.New("period is being modified concurrently; retry")
)

// UnavailablePeriod is a window the provider has blocked off.
type UnavailablePeriod struct {
	ID        uuid.UUID `json:"id"`
	ProviderID uuid.UUID `json:"provider_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Reason    *string   `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUnavailablePeriodInput is the request payload for blocking a window.
type CreateUnavailablePeriodInput struct {
	StartsAt time.Time `json:"starts_at" binding:"required"`
	EndsAt   time.Time `json:"ends_at" binding:"required"`
	Reason   *string   `json:"reason,omitempty"`
}
