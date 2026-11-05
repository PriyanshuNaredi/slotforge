package bookings

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mockBookingService struct {
	result *BookingListResult
	err    error

	calledCustomerID uuid.UUID
	calledInput      BookingListInput

	// Create / Get booking configuration.
	booking                 *Booking
	bookingErr              error
	calledBookingCustomerID uuid.UUID
	calledCreateInput       CreateBookingInput
	calledGetID             uuid.UUID
	calledGetCustomerID     uuid.UUID
	calledIsAdmin           bool

	// Discovery configuration.
	searchResult      *SlotSearchResult
	searchErr         error
	calledSearchInput SlotSearchInput

	slotResult   *AvailableSlot
	slotErr      error
	calledSlotID uuid.UUID
}

func (m *mockBookingService) ListCustomerBookings(
	ctx context.Context,
	customerID uuid.UUID,
	input BookingListInput,
) (*BookingListResult, error) {
	m.calledCustomerID = customerID
	m.calledInput = input
	return m.result, m.err
}

func (m *mockBookingService) CreateBooking(
	ctx context.Context,
	customerID uuid.UUID,
	input CreateBookingInput,
) (*Booking, error) {
	m.calledBookingCustomerID = customerID
	m.calledCreateInput = input
	return m.booking, m.bookingErr
}

func (m *mockBookingService) GetBooking(
	ctx context.Context,
	id, customerID uuid.UUID,
	isAdmin bool,
) (*Booking, error) {
	m.calledGetID = id
	m.calledGetCustomerID = customerID
	m.calledIsAdmin = isAdmin
	return m.booking, m.bookingErr
}

func (m *mockBookingService) SearchAvailableSlotsWithCount(
	ctx context.Context,
	input SlotSearchInput,
) (*SlotSearchResult, error) {
	m.calledSearchInput = input
	return m.searchResult, m.searchErr
}

func (m *mockBookingService) GetAvailableSlot(
	ctx context.Context,
	slotID uuid.UUID,
) (*AvailableSlot, error) {
	m.calledSlotID = slotID
	return m.slotResult, m.slotErr
}

// errNoRows is a shared sentinel so handler tests can exercise the
// not-found mapping without a database.
var errNoRows = pgx.ErrNoRows

func (m *mockBookingService) CancelBooking(
	ctx context.Context,
	bookingID uuid.UUID,
	customerID uuid.UUID,
) (*Booking, error) {
	return nil, nil
}

func (m *mockBookingService) RescheduleBooking(
	ctx context.Context,
	bookingID uuid.UUID,
	customerID uuid.UUID,
	newSlotID uuid.UUID,
) (*Booking, error) {
	return nil, nil
}
