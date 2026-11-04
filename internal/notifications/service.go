package notifications

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// var (
// 	ErrInvalidPagination    = errors.New("invalid pagination parameters")
// 	ErrNotificationNotFound = errors.New("notification not found")
// )

var ErrInvalidPagination = errors.New("invalid pagination parameters")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(
	ctx context.Context,
	userID uuid.UUID,
	input ListInput,
) (*NotificationListResult, error) {
	if userID == uuid.Nil {
		return nil, errors.New("invalid user ID")
	}

	// Defaults are applied by the handler, but validate here too.
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > 100 || input.Offset < 0 {
		return nil, ErrInvalidPagination
	}

	return s.repo.ListByUser(ctx, userID, input)
}

func (s *Service) CountUnread(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	if userID == uuid.Nil {
		return 0, errors.New("invalid user ID")
	}

	return s.repo.CountUnread(ctx, userID)
}

func (s *Service) MarkAsRead(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) (*Notification, error) {
	if userID == uuid.Nil || notificationID == uuid.Nil {
		return nil, errors.New("invalid user or notification ID")
	}

	notification, err := s.repo.MarkAsRead(ctx, userID, notificationID)
	if err != nil {
		if errors.Is(err, ErrNotificationNotFound) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}

	return notification, nil
}

func (s *Service) MarkAllAsRead(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	if userID == uuid.Nil {
		return 0, errors.New("invalid user ID")
	}

	return s.repo.MarkAllAsRead(ctx, userID)
}
