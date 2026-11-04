package notifications

import (
	"context"
	"testing"
)

// TestEmailWorkerIntegration runs one real ClaimBatch/process/MarkSent cycle
// against PostgreSQL with a stub sender, verifying the worker and the
// delivery repository work together end to end.
func TestEmailWorkerIntegration(t *testing.T) {
	pool := newNotificationTestPool(t)
	ctx := context.Background()

	userID := createNotificationTestUser(t, pool)
	notificationID := createTestNotification(
		t, pool, userID, "Worker integration test",
	)
	delivery := createTestDelivery(
		t, pool, notificationID, "worker-integration@example.com",
	)

	// Make this delivery the oldest claimable job so the batch of 1 claims it
	// even when other tests left pending rows behind.
	if _, err := pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET available_at = NOW() - INTERVAL '1 hour'
		WHERE id = $1
	`, delivery.ID); err != nil {
		t.Fatalf("requeue delivery for immediate processing: %v", err)
	}

	sender := &stubEmailSender{}

	worker, err := NewEmailWorker(
		NewDeliveryRepository(pool),
		NewRepository(pool),
		sender,
		EmailWorkerConfig{BatchSize: 1},
	)
	if err != nil {
		t.Fatalf("create email worker: %v", err)
	}

	worker.processBatch(ctx)

	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sender.messages))
	}

	if sender.messages[0].To != "worker-integration@example.com" {
		t.Errorf("unexpected recipient: %s", sender.messages[0].To)
	}

	if sender.messages[0].Subject != "Worker integration test" {
		// The integration notification uses type 'test_notification', so the
		// subject falls back to the notification title.
		t.Errorf("unexpected subject: %s", sender.messages[0].Subject)
	}

	updated := getTestDelivery(t, pool, delivery.ID)

	if updated.Status != DeliverySent {
		t.Errorf("expected sent status, got %s", updated.Status)
	}

	if updated.SentAt == nil {
		t.Error("expected sent_at to be set")
	}
}
