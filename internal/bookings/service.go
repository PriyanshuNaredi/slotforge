package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/priyanshunaredi/slotforge/internal/cache"
	// "github.com/priyanshunaredi/slotforge/internal/notifications"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	repo  *Repository
	redis *redis.Client
}

const slotSearchCacheTTL = 30 * time.Second

func NewService(repo *Repository, redisClients ...*redis.Client) *Service {
	service := &Service{
		repo: repo,
	}

	if len(redisClients) > 0 {
		service.redis = redisClients[0]
	}

	return service
}

func slotSearchCacheKey(input SlotSearchInput) string {
	providerID := "all"
	if input.ProviderID != nil {
		providerID = input.ProviderID.String()
	}

	from := "none"
	if input.From != nil {
		from = input.From.UTC().Format(time.RFC3339Nano)
	}

	to := "none"
	if input.To != nil {
		to = input.To.UTC().Format(time.RFC3339Nano)
	}

	return fmt.Sprintf(
		"slot-search:v1:provider=%s:from=%s:to=%s:limit=%d:offset=%d",
		providerID,
		from,
		to,
		input.Limit,
		input.Offset,
	)
}

// func (s *Service) CreateBooking(
// 	ctx context.Context,
// 	customerID uuid.UUID,
// 	input CreateBookingInput,
// ) (*Booking, error) {
// 	if customerID == uuid.Nil {
// 		return nil, errors.New("invalid customer ID")
// 	}

// 	if input.SlotID == uuid.Nil {
// 		return nil, errors.New("slot_id is required")
// 	}

// 	return s.repo.Create(ctx, customerID, input)
// }

func (s *Service) GetBooking(
	ctx context.Context,
	id uuid.UUID,
	customerID uuid.UUID,
	isAdmin bool,
) (*Booking, error) {
	if id == uuid.Nil || customerID == uuid.Nil {
		return nil, errors.New("invalid booking or customer ID")
	}

	return s.repo.GetByIDForUser(ctx, id, customerID, isAdmin)
}

func (s *Service) SearchAvailableSlots(
	ctx context.Context,
	input SlotSearchInput,
) ([]AvailableSlot, error) {
	return s.repo.SearchAvailableSlots(ctx, input)
}

func (s *Service) SearchAvailableSlotsWithCount(
	ctx context.Context,
	input SlotSearchInput,
) (*SlotSearchResult, error) {

	// Attempt to retrieve the result from Redis.
	if s.redis != nil {
		cacheKey := slotSearchCacheKey(input)

		cached, err := s.redis.Get(ctx, cacheKey).Result()

		switch {
		case err == nil:
			var result SlotSearchResult

			if unmarshalErr := json.Unmarshal(
				[]byte(cached),
				&result,
			); unmarshalErr == nil {
				return &result, nil
			}

			log.Printf("WARNING: invalid slot search cache entry: %v", cacheKey)

			// Remove corrupted cached data.
			if deleteErr := s.redis.Del(ctx, cacheKey).Err(); deleteErr != nil {
				log.Printf("WARNING: could not delete invalid cache entry: %v", deleteErr)
			}

		case errors.Is(err, redis.Nil):
			// Cache miss. Continue to PostgreSQL.

		default:
			log.Printf("WARNING: Redis cache read failed: %v", err)
		}
	}

	// Query PostgreSQL on cache miss or Redis failure.
	slots, err := s.repo.SearchAvailableSlots(ctx, input)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountAvailableSlots(ctx, input)
	if err != nil {
		return nil, err
	}

	result := &SlotSearchResult{
		Slots:  slots,
		Limit:  input.Limit,
		Offset: input.Offset,
		Total:  total,
	}

	// Populate Redis without failing the API if caching fails.
	if s.redis != nil {
		cacheKey := slotSearchCacheKey(input)

		payload, err := json.Marshal(result)
		if err != nil {
			log.Printf("WARNING: could not serialize slot search result: %v", err)
			return result, nil
		}

		if err := s.redis.Set(
			ctx,
			cacheKey,
			payload,
			slotSearchCacheTTL,
		).Err(); err != nil {
			log.Printf("WARNING: Redis cache write failed: %v", err)
		}
	}

	return result, nil
}

func (s *Service) GetAvailableSlot(
	ctx context.Context,
	slotID uuid.UUID,
) (*AvailableSlot, error) {
	return s.repo.GetAvailableSlotByID(ctx, slotID)
}

func (s *Service) ListCustomerBookings(
	ctx context.Context,
	customerID uuid.UUID,
	input BookingListInput,
) (*BookingListResult, error) {

	bookings, err := s.repo.ListByCustomer(ctx, customerID, input)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountByCustomer(ctx, customerID, input)
	if err != nil {
		return nil, err
	}

	return &BookingListResult{
		Bookings: bookings,
		Limit:    input.Limit,
		Offset:   input.Offset,
		Total:    total,
	}, nil
}

func (s *Service) CancelBooking(
	ctx context.Context,
	bookingID uuid.UUID,
	customerID uuid.UUID,
) (*Booking, error) {
	if bookingID == uuid.Nil || customerID == uuid.Nil {
		return nil, errors.New("invalid booking or customer ID")
	}

	booking, err := s.repo.CancelByCustomer(
		ctx,
		bookingID,
		customerID,
	)
	if err != nil {
		return nil, err
	}

	s.invalidateSlotSearchCache()

	return booking, nil
}

// RescheduleBooking moves a confirmed booking to a new slot. Both the old
// and the new slot's availability change, so the slot-search cache must be
// invalidated.
func (s *Service) RescheduleBooking(
	ctx context.Context,
	bookingID uuid.UUID,
	customerID uuid.UUID,
	newSlotID uuid.UUID,
) (*Booking, error) {
	if bookingID == uuid.Nil || customerID == uuid.Nil {
		return nil, errors.New("invalid booking or customer ID")
	}

	if newSlotID == uuid.Nil {
		return nil, errors.New("slot_id is required")
	}

	booking, err := s.repo.RescheduleByCustomer(
		ctx,
		bookingID,
		customerID,
		newSlotID,
	)
	if err != nil {
		return nil, err
	}

	s.invalidateSlotSearchCache()

	return booking, nil
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
func (s *Service) CreateBooking(
	ctx context.Context,
	customerID uuid.UUID,
	input CreateBookingInput,
) (*Booking, error) {
	if customerID == uuid.Nil {
		return nil, errors.New("invalid customer ID")
	}

	if input.SlotID == uuid.Nil {
		return nil, errors.New("slot_id is required")
	}

	booking, err := s.repo.Create(ctx, customerID, input)
	if err != nil {
		return nil, err
	}

	s.invalidateSlotSearchCache()

	return booking, nil
}
