package providers

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSlotNotFound         = errors.New("availability slot not found")
	ErrInvalidSlotTime      = errors.New("slot end time must be after start time")
	ErrSlotOverlap          = errors.New("slot overlaps with an existing open slot")
	ErrSlotHasActiveBooking = errors.New("cannot close a slot with an active booking")
	// ErrSlotBlockedByUnavailability is returned when explicitly requested
	// availability overlaps a blocked unavailable period. The recurring batch
	// path instead skips such windows (reported in the response counts).
	ErrSlotBlockedByUnavailability = errors.New("slot overlaps an unavailable period")
)

// Slot represents an appointment slot offered by a provider.
type Slot struct {
	ID         uuid.UUID `json:"id"`
	ProviderID uuid.UUID `json:"provider_id"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateSlotInput contains the data required to create an availability slot.
type CreateSlotInput struct {
	StartsAt time.Time `json:"starts_at" binding:"required"`
	EndsAt   time.Time `json:"ends_at" binding:"required"`
}

const (
	SlotStatusOpen   = "open"
	SlotStatusClosed = "closed"
)
