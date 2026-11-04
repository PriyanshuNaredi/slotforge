package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type retryCall struct {
	deliveryID uuid.UUID
	cause      string
}

type stubDeliveryStore struct {
	claimed     []Delivery
	claimErr    error
	markSentErr error

	markSentCalls []uuid.UUID
	retryCalls    []retryCall
}

func (s *stubDeliveryStore) ClaimBatch(
	ctx context.Context,
	batchSize int,
) ([]Delivery, error) {
	if s.claimErr != nil {
		return nil, s.claimErr
	}

	return s.claimed, nil
}

func (s *stubDeliveryStore) MarkSent(
	ctx context.Context,
	deliveryID uuid.UUID,
) error {
	s.markSentCalls = append(s.markSentCalls, deliveryID)
	return s.markSentErr
}

func (s *stubDeliveryStore) ScheduleRetry(
	ctx context.Context,
	deliveryID uuid.UUID,
	deliveryError string,
) error {
	s.retryCalls = append(s.retryCalls, retryCall{
		deliveryID: deliveryID,
		cause:      deliveryError,
	})
	return nil
}

func (s *stubDeliveryStore) RecoverStaleClaims(
	ctx context.Context,
	staleAfter time.Duration,
) (int64, error) {
	return 0, nil
}

type stubNotificationStore struct {
	notification *Notification
	err          error
}

func (s *stubNotificationStore) GetByIDForDelivery(
	ctx context.Context,
	notificationID uuid.UUID,
) (*Notification, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.notification, nil
}

type stubEmailSender struct {
	err      error
	messages []EmailMessage
}

func (s *stubEmailSender) SendEmail(
	ctx context.Context,
	message EmailMessage,
) error {
	s.messages = append(s.messages, message)
	return s.err
}

// failingOnceSender fails the first send and succeeds afterwards, to verify
// one failed job does not stop the rest of the batch.
type failingOnceSender struct {
	calls    int
	messages []EmailMessage
}

func (s *failingOnceSender) SendEmail(
	ctx context.Context,
	message EmailMessage,
) error {
	s.calls++
	s.messages = append(s.messages, message)

	if s.calls == 1 {
		return errors.New("temporary provider failure")
	}

	return nil
}

// cancellingSender cancels the context on the first send, to verify in-flight
// batch processing stops at the cancellation point.
type cancellingSender struct {
	cancel context.CancelFunc
	calls  int
}

func (s *cancellingSender) SendEmail(
	ctx context.Context,
	message EmailMessage,
) error {
	s.calls++
	s.cancel()
	return nil
}

func newTestDelivery(id uuid.UUID) Delivery {
	return Delivery{
		ID:             id,
		NotificationID: uuid.New(),
		RecipientEmail: "customer@example.com",
		Status:         DeliveryProcessing,
	}
}

func newTestWorker(
	t *testing.T,
	deliveries DeliveryStore,
	notifications NotificationStore,
	sender EmailSender,
) *EmailWorker {
	t.Helper()

	worker, err := NewEmailWorker(
		deliveries,
		notifications,
		sender,
		EmailWorkerConfig{},
	)
	if err != nil {
		t.Fatalf("create email worker: %v", err)
	}

	return worker
}

func TestEmailWorkerSuccessfulDeliveryMarksSent(t *testing.T) {
	deliveryID := uuid.New()

	deliveries := &stubDeliveryStore{claimed: []Delivery{newTestDelivery(deliveryID)}}
	notifications := &stubNotificationStore{notification: &Notification{
		ID:      uuid.New(),
		Type:    "booking_confirmed",
		Title:   "Booking confirmed",
		Message: "Your booking has been confirmed.",
	}}
	sender := &stubEmailSender{}

	newTestWorker(t, deliveries, notifications, sender).
		processBatch(context.Background())

	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sender.messages))
	}

	if sender.messages[0].To != "customer@example.com" {
		t.Errorf("unexpected recipient: %s", sender.messages[0].To)
	}

	if sender.messages[0].Subject != "Your booking is confirmed" {
		t.Errorf("unexpected subject: %s", sender.messages[0].Subject)
	}

	if len(deliveries.markSentCalls) != 1 ||
		deliveries.markSentCalls[0] != deliveryID {
		t.Errorf("expected MarkSent for %s, got %v",
			deliveryID, deliveries.markSentCalls)
	}

	if len(deliveries.retryCalls) != 0 {
		t.Errorf("expected no retry calls, got %v", deliveries.retryCalls)
	}
}

func TestEmailWorkerProviderFailureSchedulesRetry(t *testing.T) {
	deliveryID := uuid.New()

	deliveries := &stubDeliveryStore{claimed: []Delivery{newTestDelivery(deliveryID)}}
	notifications := &stubNotificationStore{notification: &Notification{
		Type:    "booking_confirmed",
		Title:   "Booking confirmed",
		Message: "Your booking has been confirmed.",
	}}
	sender := &stubEmailSender{err: errors.New("provider unavailable")}

	newTestWorker(t, deliveries, notifications, sender).
		processBatch(context.Background())

	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 send attempt, got %d", len(sender.messages))
	}

	if len(deliveries.retryCalls) != 1 {
		t.Fatalf("expected 1 retry call, got %d", len(deliveries.retryCalls))
	}

	if deliveries.retryCalls[0].deliveryID != deliveryID {
		t.Errorf("expected retry for %s, got %s",
			deliveryID, deliveries.retryCalls[0].deliveryID)
	}

	if deliveries.retryCalls[0].cause == "" {
		t.Error("expected the retry cause to be stored")
	}

	if len(deliveries.markSentCalls) != 0 {
		t.Error("MarkSent must not be called after a failed send")
	}
}

func TestEmailWorkerOneFailedJobDoesNotStopBatch(t *testing.T) {
	first := newTestDelivery(uuid.New())
	second := newTestDelivery(uuid.New())

	deliveries := &stubDeliveryStore{claimed: []Delivery{first, second}}
	notifications := &stubNotificationStore{notification: &Notification{
		Type:    "booking_confirmed",
		Title:   "Booking confirmed",
		Message: "Your booking has been confirmed.",
	}}
	sender := &failingOnceSender{}

	newTestWorker(t, deliveries, notifications, sender).
		processBatch(context.Background())

	if len(sender.messages) != 2 {
		t.Fatalf("expected both deliveries to be attempted, got %d",
			len(sender.messages))
	}

	if len(deliveries.retryCalls) != 1 ||
		deliveries.retryCalls[0].deliveryID != first.ID {
		t.Errorf("expected one retry for the first delivery, got %v",
			deliveries.retryCalls)
	}

	if len(deliveries.markSentCalls) != 1 ||
		deliveries.markSentCalls[0] != second.ID {
		t.Errorf("expected MarkSent for the second delivery, got %v",
			deliveries.markSentCalls)
	}
}

func TestEmailWorkerNotificationLookupFailureIsSafe(t *testing.T) {
	deliveryID := uuid.New()

	deliveries := &stubDeliveryStore{claimed: []Delivery{newTestDelivery(deliveryID)}}
	notifications := &stubNotificationStore{err: errors.New("database down")}
	sender := &stubEmailSender{}

	newTestWorker(t, deliveries, notifications, sender).
		processBatch(context.Background())

	if len(sender.messages) != 0 {
		t.Errorf("no email should be sent without notification content, got %d",
			len(sender.messages))
	}

	if len(deliveries.retryCalls) != 1 ||
		deliveries.retryCalls[0].deliveryID != deliveryID {
		t.Errorf("expected one retry for %s, got %v",
			deliveryID, deliveries.retryCalls)
	}
}

func TestEmailWorkerMarkSentFailureDoesNotRetry(t *testing.T) {
	deliveryID := uuid.New()

	deliveries := &stubDeliveryStore{
		claimed:     []Delivery{newTestDelivery(deliveryID)},
		markSentErr: errors.New("database down"),
	}
	notifications := &stubNotificationStore{notification: &Notification{
		Type:    "booking_confirmed",
		Title:   "Booking confirmed",
		Message: "Your booking has been confirmed.",
	}}
	sender := &stubEmailSender{}

	newTestWorker(t, deliveries, notifications, sender).
		processBatch(context.Background())

	// The email was accepted by the provider, so the job must not be
	// retried: that would deliver a duplicate email.
	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sender.messages))
	}

	if len(deliveries.retryCalls) != 0 {
		t.Errorf("retry must not be scheduled after a successful send, got %v",
			deliveries.retryCalls)
	}
}

func TestEmailWorkerClaimFailureIsHandled(t *testing.T) {
	deliveries := &stubDeliveryStore{claimErr: errors.New("database down")}
	sender := &stubEmailSender{}

	newTestWorker(t, deliveries, &stubNotificationStore{}, sender).
		processBatch(context.Background())

	if len(sender.messages) != 0 {
		t.Errorf("expected no sends, got %d", len(sender.messages))
	}

	if len(deliveries.retryCalls) != 0 {
		t.Errorf("expected no retries, got %v", deliveries.retryCalls)
	}
}

func TestEmailWorkerProcessBatchRespectsCancelledContext(t *testing.T) {
	deliveries := &stubDeliveryStore{claimed: []Delivery{newTestDelivery(uuid.New())}}
	notifications := &stubNotificationStore{notification: &Notification{
		Type:    "booking_confirmed",
		Title:   "Booking confirmed",
		Message: "Your booking has been confirmed.",
	}}
	sender := &stubEmailSender{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	newTestWorker(t, deliveries, notifications, sender).processBatch(ctx)

	if len(sender.messages) != 0 {
		t.Errorf("expected no sends after cancellation, got %d",
			len(sender.messages))
	}

	if len(deliveries.markSentCalls) != 0 {
		t.Errorf("expected no MarkSent calls after cancellation, got %v",
			deliveries.markSentCalls)
	}
}

func TestEmailWorkerStopsBatchAtCancellation(t *testing.T) {
	first := newTestDelivery(uuid.New())
	second := newTestDelivery(uuid.New())

	deliveries := &stubDeliveryStore{claimed: []Delivery{first, second}}
	notifications := &stubNotificationStore{notification: &Notification{
		Type:    "booking_confirmed",
		Title:   "Booking confirmed",
		Message: "Your booking has been confirmed.",
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender := &cancellingSender{cancel: cancel}

	newTestWorker(t, deliveries, notifications, sender).processBatch(ctx)

	if sender.calls != 1 {
		t.Errorf("expected processing to stop after the first send, got %d sends",
			sender.calls)
	}

	if len(deliveries.markSentCalls) != 1 ||
		deliveries.markSentCalls[0] != first.ID {
		t.Errorf("expected MarkSent only for the first delivery, got %v",
			deliveries.markSentCalls)
	}
}

func TestEmailWorkerRunReturnsOnContextCancellation(t *testing.T) {
	worker, err := NewEmailWorker(
		&stubDeliveryStore{},
		&stubNotificationStore{notification: &Notification{
			Type:    "booking_confirmed",
			Title:   "Booking confirmed",
			Message: "Your booking has been confirmed.",
		}},
		&stubEmailSender{},
		EmailWorkerConfig{
			PollInterval:     10 * time.Millisecond,
			RecoveryInterval: 10 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("create email worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestEmailWorkerConfigValidation(t *testing.T) {
	tests := []struct {
		name       string
		batchSize  int
		staleAfter time.Duration
		wantErr    bool
	}{
		{"defaults are safe", 10, 5 * time.Minute, false},
		{"batch fits within stale timeout", 19, 5 * time.Minute, false},
		{"batch exactly matches stale timeout", 20, 5 * time.Minute, true},
		{"batch outlives stale timeout", 100, 5 * time.Minute, true},
		{"zero values normalize to safe defaults", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEmailWorker(
				&stubDeliveryStore{},
				&stubNotificationStore{},
				&stubEmailSender{},
				EmailWorkerConfig{
					BatchSize:  tt.batchSize,
					StaleAfter: tt.staleAfter,
				},
			)

			if tt.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestEmailWorkerAppliesDefaultConfig(t *testing.T) {
	worker, err := NewEmailWorker(
		&stubDeliveryStore{},
		&stubNotificationStore{},
		&stubEmailSender{},
		EmailWorkerConfig{},
	)
	if err != nil {
		t.Fatalf("create email worker: %v", err)
	}

	if worker.batchSize != defaultWorkerBatchSize {
		t.Errorf("expected default batch size %d, got %d",
			defaultWorkerBatchSize, worker.batchSize)
	}

	if worker.pollInterval != defaultWorkerPollInterval {
		t.Errorf("expected default poll interval, got %s", worker.pollInterval)
	}

	if worker.recoveryInterval != defaultWorkerRecoveryInterval {
		t.Errorf("expected default recovery interval, got %s",
			worker.recoveryInterval)
	}

	if worker.staleAfter != defaultWorkerStaleAfter {
		t.Errorf("expected default stale after, got %s", worker.staleAfter)
	}
}
