package bookings

import (
	"time"

	"github.com/google/uuid"
)

// AvailableSlot represents an appointment customers can book.
type AvailableSlot struct {
	ID          uuid.UUID `json:"id"`
	ProviderID  uuid.UUID `json:"provider_id"`
	DisplayName string    `json:"provider_name"`
	Timezone    string    `json:"timezone"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
}

// SlotSearchInput contains customer search filters.
type SlotSearchInput struct {
	ProviderID *uuid.UUID
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

type SlotSearchResult struct {
	Slots  []AvailableSlot `json:"slots"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
	Total  int64           `json:"total"`
}
