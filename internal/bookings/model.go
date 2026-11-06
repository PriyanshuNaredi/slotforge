package bookings

import (
	"time"

	"github.com/google/uuid"
)

type BookingStatus string

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusCancelled BookingStatus = "cancelled"
	StatusCompleted BookingStatus = "completed"
	StatusNoShow    BookingStatus = "no_show"
)

type Booking struct {
	ID         uuid.UUID     `json:"id"`
	SlotID     uuid.UUID     `json:"slot_id"`
	CustomerID uuid.UUID     `json:"customer_id"`
	Status     BookingStatus `json:"status"`
	Notes      *string       `json:"notes,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// BookingDetail is the response shape for GET /bookings/:id. It joins the
// booking with its slot and the provider/customer identities so historical
// bookings stay displayable after their slot closes or passes. It
// deliberately omits internal account fields (password hash, role,
// is_active) and the provider's email; the customer email follows the
// existing authorized pattern of the provider bookings list.
type BookingDetail struct {
	ID               uuid.UUID     `json:"id"`
	SlotID           uuid.UUID     `json:"slot_id"`
	CustomerID       uuid.UUID     `json:"customer_id"`
	Status           BookingStatus `json:"status"`
	Notes            *string       `json:"notes,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	SlotStatus       string        `json:"slot_status"`
	StartsAt         time.Time     `json:"starts_at"`
	EndsAt           time.Time     `json:"ends_at"`
	ProviderID       uuid.UUID     `json:"provider_id"`
	ProviderName     string        `json:"provider_display_name"`
	ProviderTimezone string        `json:"provider_timezone"`
	CustomerEmail    string        `json:"customer_email"`
}

type CreateBookingInput struct {
	SlotID uuid.UUID `json:"slot_id" binding:"required"`
	Notes  *string   `json:"notes,omitempty"`
}

type RescheduleInput struct {
	SlotID uuid.UUID `json:"slot_id" binding:"required"`
}
