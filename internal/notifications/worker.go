package notifications

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// DeliveryStore abstracts the delivery queue so the worker can be tested
// without PostgreSQL. *DeliveryRepository implements it.
type DeliveryStore interface {
	ClaimBatch(ctx context.Context, batchSize int) ([]Delivery, error)
	MarkSent(ctx context.Context, deliveryID uuid.UUID) error
	ScheduleRetry(
		ctx context.Context,
		deliveryID uuid.UUID,
		deliveryError string,
	) error
	RecoverStaleClaims(
		ctx context.Context,
		staleAfter time.Duration,
	) (int64, error)
}

// NotificationStore abstracts notification lookups for the worker.
// *Repository implements it.
type NotificationStore interface {
	GetByIDForDelivery(
		ctx context.Context,
		notificationID uuid.UUID,
	) (*Notification, error)
}

// EmailWorkerConfig holds the tunable worker settings. Zero values are
// replaced with defaults.
type EmailWorkerConfig struct {
	BatchSize        int
	PollInterval     time.Duration
	RecoveryInterval time.Duration
	StaleAfter       time.Duration
}

// EmailWorker delivers pending notification emails in the background.
//
// Delivery is at-least-once, not exactly-once. If the provider accepted an
// email but MarkSent fails, or the worker crashes mid-claim, stale-claim
// recovery requeues the job and the recipient may receive a duplicate email.
type EmailWorker struct {
	deliveries       DeliveryStore
	notifications    NotificationStore
	sender           EmailSender
	batchSize        int
	pollInterval     time.Duration
	recoveryInterval time.Duration
	staleAfter       time.Duration
}

const (
	defaultWorkerBatchSize        = 10
	defaultWorkerPollInterval     = 15 * time.Second
	defaultWorkerRecoveryInterval = time.Minute
	defaultWorkerStaleAfter       = 5 * time.Minute
)

func NewEmailWorker(
	deliveries DeliveryStore,
	notificationStore NotificationStore,
	sender EmailSender,
	config EmailWorkerConfig,
) (*EmailWorker, error) {
	if config.BatchSize < 1 {
		config.BatchSize = defaultWorkerBatchSize
	}

	if config.PollInterval <= 0 {
		config.PollInterval = defaultWorkerPollInterval
	}

	if config.RecoveryInterval <= 0 {
		config.RecoveryInterval = defaultWorkerRecoveryInterval
	}

	if config.StaleAfter <= 0 {
		config.StaleAfter = defaultWorkerStaleAfter
	}

	// Batches are processed sequentially and each send can take up to
	// emailSendTimeout, so a claimed job can sit in this worker's batch for
	// up to batchSize * emailSendTimeout. That must stay below staleAfter,
	// or another worker will reclaim jobs that are still in flight here.
	maxBatchDuration := time.Duration(config.BatchSize) * emailSendTimeout
	if config.StaleAfter <= maxBatchDuration {
		return nil, fmt.Errorf(
			"invalid email worker config: batch size %d can take up to %s to process, which exceeds the stale-claim timeout %s; reduce EMAIL_WORKER_BATCH_SIZE or increase EMAIL_WORKER_STALE_AFTER",
			config.BatchSize,
			maxBatchDuration,
			config.StaleAfter,
		)
	}

	return &EmailWorker{
		deliveries:       deliveries,
		notifications:    notificationStore,
		sender:           sender,
		batchSize:        config.BatchSize,
		pollInterval:     config.PollInterval,
		recoveryInterval: config.RecoveryInterval,
		staleAfter:       config.StaleAfter,
	}, nil
}

// Run processes deliveries until ctx is cancelled. It is intended to run in
// its own goroutine and returns promptly after cancellation. Jobs claimed but
// unfinished at shutdown are returned to the queue by stale-claim recovery.
func (w *EmailWorker) Run(ctx context.Context) {
	pollTicker := time.NewTicker(w.pollInterval)
	defer pollTicker.Stop()

	recoveryTicker := time.NewTicker(w.recoveryInterval)
	defer recoveryTicker.Stop()

	// Deliver anything queued before the first tick.
	w.processBatch(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			w.processBatch(ctx)
		case <-recoveryTicker.C:
			w.recoverStaleClaims(ctx)
		}
	}
}

func (w *EmailWorker) recoverStaleClaims(ctx context.Context) {
	recovered, err := w.deliveries.RecoverStaleClaims(ctx, w.staleAfter)
	if err != nil {
		log.Printf("email worker: recover stale claims: %v", err)
		return
	}

	if recovered > 0 {
		log.Printf(
			"email worker: recovered %d stale delivery claim(s)",
			recovered,
		)
	}
}

func (w *EmailWorker) processBatch(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	deliveries, err := w.deliveries.ClaimBatch(ctx, w.batchSize)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("email worker: claim delivery batch: %v", err)
		}
		return
	}

	for i := range deliveries {
		// Remaining claimed jobs are handled by stale-claim recovery.
		if ctx.Err() != nil {
			return
		}

		w.processOne(ctx, &deliveries[i])
	}
}

func (w *EmailWorker) processOne(
	ctx context.Context,
	delivery *Delivery,
) {
	notification, err := w.notifications.GetByIDForDelivery(
		ctx,
		delivery.NotificationID,
	)
	if err != nil {
		if ctx.Err() != nil {
			return
		}

		log.Printf(
			"email worker: lookup notification for delivery %s: %v",
			delivery.ID,
			err,
		)

		w.scheduleRetry(ctx, delivery, "notification lookup failed")
		return
	}

	content := renderEmailContent(notification)

	if err := w.sender.SendEmail(ctx, EmailMessage{
		To:      delivery.RecipientEmail,
		Subject: content.Subject,
		HTML:    content.HTML,
		Text:    content.Text,
	}); err != nil {
		if ctx.Err() != nil {
			// Shutdown: leave the claim for stale-claim recovery.
			return
		}

		log.Printf(
			"email worker: send email for delivery %s failed: %v",
			delivery.ID,
			err,
		)

		w.scheduleRetry(ctx, delivery, err.Error())
		return
	}

	if err := w.deliveries.MarkSent(ctx, delivery.ID); err != nil {
		// The provider accepted the email, so retrying would send a
		// duplicate. Leave the claim in place for stale-claim recovery.
		log.Printf(
			"email worker: delivery %s was sent but could not be marked sent: %v; the job will be recovered and may be delivered again",
			delivery.ID,
			err,
		)
		return
	}

	log.Printf("email worker: delivered notification email %s", delivery.ID)
}

func (w *EmailWorker) scheduleRetry(
	ctx context.Context,
	delivery *Delivery,
	cause string,
) {
	if err := w.deliveries.ScheduleRetry(
		ctx,
		delivery.ID,
		truncateErrorText(cause),
	); err != nil {
		log.Printf(
			"email worker: schedule retry for delivery %s: %v",
			delivery.ID,
			err,
		)
	}
}
