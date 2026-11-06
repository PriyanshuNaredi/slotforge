package providers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/cache"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	repo  *Repository
	redis *redis.Client
}

func NewService(
	repo *Repository,
	redisClients ...*redis.Client,
) *Service {
	service := &Service{
		repo: repo,
	}

	if len(redisClients) > 0 {
		service.redis = redisClients[0]
	}

	return service
}

func (s *Service) invalidateSlotSearchCache() {
	if s.redis == nil {
		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	cache.InvalidateSlotSearch(ctx, s.redis)
}

func (s *Service) CreateProfile(
	ctx context.Context,
	userID uuid.UUID,
	input CreateProfileInput,
) (*Profile, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)

	if userID == uuid.Nil {
		return nil, errors.New("invalid user ID")
	}

	if len(input.DisplayName) < 2 || len(input.DisplayName) > 150 {
		return nil, errors.New("display name must be between 2 and 150 characters")
	}

	if input.Timezone == "" {
		input.Timezone = "UTC"
	}

	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return nil, errors.New("invalid timezone")
	}

	return s.repo.Create(ctx, userID, input)
}

func (s *Service) GetMyProfile(
	ctx context.Context,
	userID uuid.UUID,
) (*Profile, error) {
	if userID == uuid.Nil {
		return nil, errors.New("invalid user ID")
	}

	return s.repo.GetByUserID(ctx, userID)
}

// ListPublicProviders returns the public provider directory with a total
// count independent of the requested page.
func (s *Service) ListPublicProviders(
	ctx context.Context,
	input ProviderListInput,
) (*ProviderListResult, error) {
	items, err := s.repo.ListPublicProviders(ctx, input)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountPublicProviders(ctx, input)
	if err != nil {
		return nil, err
	}

	return &ProviderListResult{
		Items:  items,
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (s *Service) GetPublicProvider(
	ctx context.Context,
	providerID uuid.UUID,
) (*PublicProvider, error) {
	if providerID == uuid.Nil {
		return nil, ErrProfileNotFound
	}

	return s.repo.GetPublicProvider(ctx, providerID)
}

func (s *Service) CreateSlot(
	ctx context.Context,
	providerID uuid.UUID,
	input CreateSlotInput,
) (*Slot, error) {
	if !input.EndsAt.After(input.StartsAt) {
		return nil, ErrInvalidSlotTime
	}

	slot, err := s.repo.CreateSlot(ctx, providerID, input)
	if err != nil {
		return nil, err
	}

	s.invalidateSlotSearchCache()

	return slot, nil
}

func (s *Service) ListSlots(
	ctx context.Context,
	providerID uuid.UUID,
) ([]Slot, error) {

	return s.repo.ListSlots(ctx, providerID)
}

func (s *Service) GetSlot(
	ctx context.Context,
	providerID uuid.UUID,
	slotID uuid.UUID,
) (*Slot, error) {

	return s.repo.GetSlot(ctx, providerID, slotID)
}

func (s *Service) CloseSlot(
	ctx context.Context,
	providerID uuid.UUID,
	slotID uuid.UUID,
) error {
	if err := s.repo.CloseSlot(ctx, providerID, slotID); err != nil {
		return err
	}

	s.invalidateSlotSearchCache()

	return nil
}

// Sentinel validation errors so the handler can distinguish client
// input mistakes from internal failures without leaking error text.
var (
	ErrInvalidBookingStatus = errors.New("invalid booking status")
	ErrInvalidDateRange     = errors.New("end date must be after start date")
	ErrInvalidLimit         = errors.New("limit must be between 1 and 100")
	ErrInvalidOffset        = errors.New("offset cannot be negative")
)

func (s *Service) ListProviderBookings(
	ctx context.Context,
	providerID uuid.UUID,
	input ProviderBookingListInput,
) (*ProviderBookingListResult, error) {
	if providerID == uuid.Nil {
		return nil, errors.New("invalid provider ID")
	}

	validStatuses := map[string]bool{
		"":          true,
		"pending":   true,
		"confirmed": true,
		"cancelled": true,
		"completed": true,
		"no_show":   true,
	}

	if !validStatuses[input.Status] {
		return nil, ErrInvalidBookingStatus
	}

	if input.From != nil && input.To != nil &&
		!input.To.After(*input.From) {
		return nil, ErrInvalidDateRange
	}

	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > 100 {
		return nil, ErrInvalidLimit
	}

	if input.Offset < 0 {
		return nil, ErrInvalidOffset
	}

	bookings, err := s.repo.ListBookings(ctx, providerID, input)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountBookings(ctx, providerID, input)
	if err != nil {
		return nil, err
	}

	return &ProviderBookingListResult{
		Bookings: bookings,
		Limit:    input.Limit,
		Offset:   input.Offset,
		Total:    total,
	}, nil
}

func (s *Service) UpdateProviderBookingStatus(
	ctx context.Context,
	providerID uuid.UUID,
	bookingID uuid.UUID,
	newStatus string,
) (*ProviderBooking, error) {
	if providerID == uuid.Nil {
		return nil, errors.New("invalid provider ID")
	}

	// uuid.Nil and any other unknown booking ID both fall through to the
	// repository, which returns ErrProviderBookingNotFound (→ 404).

	validStatuses := map[string]bool{
		"confirmed": true,
		"cancelled": true,
		"completed": true,
		"no_show":   true,
	}

	// Pending is supported as a current state, but providers
	// cannot request a transition back to pending.
	if !validStatuses[newStatus] {
		return nil, ErrInvalidBookingStatus
	}

	booking, err := s.repo.UpdateBookingStatus(
		ctx,
		providerID,
		bookingID,
		newStatus,
	)
	if err != nil {
		return nil, err
	}

	s.invalidateSlotSearchCache()

	return booking, nil
}
