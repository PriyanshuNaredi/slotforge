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

type CreateBookingInput struct {
	SlotID uuid.UUID `json:"slot_id" binding:"required"`
	Notes  *string   `json:"notes,omitempty"`
}

type RescheduleInput struct {
	SlotID uuid.UUID `json:"slot_id" binding:"required"`
}
