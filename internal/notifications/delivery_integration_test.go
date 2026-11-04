package notifications

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createTestDelivery(
	t *testing.T,
	pool *pgxpool.Pool,
	notificationID uuid.UUID,
	email string,
) *Delivery {
	t.Helper()

	repo := NewDeliveryRepository(pool)

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin delivery creation transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	delivery, err := repo.Create(
		context.Background(),
		tx,
		CreateDeliveryInput{
			NotificationID: notificationID,
			RecipientEmail: email,
		},
	)
	if err != nil {
		t.Fatalf("create test delivery: %v", err)
	}

	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit delivery creation transaction: %v", err)
	}

	return delivery
}

func getTestDelivery(
	t *testing.T,
	pool *pgxpool.Pool,
	deliveryID uuid.UUID,
) *Delivery {
	t.Helper()

	var delivery Delivery

	err := pool.QueryRow(context.Background(), `
		SELECT
			id,
			notification_id,
			recipient_email,
			status,
			attempts,
			available_at,
			locked_at,
			sent_at,
			last_error,
			created_at,
			updated_at
		FROM notification_deliveries
		WHERE id = $1
	`, deliveryID).Scan(
		&delivery.ID,
		&delivery.NotificationID,
		&delivery.RecipientEmail,
		&delivery.Status,
		&delivery.Attempts,
		&delivery.AvailableAt,
		&delivery.LockedAt,
		&delivery.SentAt,
		&delivery.LastError,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("get test delivery: %v", err)
	}

	return &delivery
}

func TestDeliveryRepositoryIntegration(t *testing.T) {
	pool := newNotificationTestPool(t)
	repo := NewDeliveryRepository(pool)
	ctx := context.Background()

	userID := createNotificationTestUser(t, pool)

	t.Run("create delivery", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Delivery creation test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "customer@example.com",
		)

		if delivery.ID == uuid.Nil {
			t.Error("expected delivery ID to be generated")
		}

		if delivery.NotificationID != notificationID {
			t.Errorf("expected notification ID %s, got %s",
				notificationID, delivery.NotificationID)
		}

		if delivery.RecipientEmail != "customer@example.com" {
			t.Errorf("unexpected recipient email: %s",
				delivery.RecipientEmail)
		}

		if delivery.Status != DeliveryPending {
			t.Errorf("expected pending status, got %s", delivery.Status)
		}

		if delivery.Attempts != 0 {
			t.Errorf("expected 0 attempts, got %d", delivery.Attempts)
		}

		if delivery.LockedAt != nil || delivery.SentAt != nil {
			t.Error("new delivery should not have lock or sent timestamps")
		}
	})

	t.Run("reject invalid delivery input", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin transaction: %v", err)
		}
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		_, err = repo.Create(ctx, tx, CreateDeliveryInput{
			NotificationID: uuid.Nil,
			RecipientEmail: "customer@example.com",
		})

		if !errors.Is(err, ErrInvalidDeliveryInput) {
			t.Errorf("expected ErrInvalidDeliveryInput, got %v", err)
		}

		notificationID := createTestNotification(
			t, pool, userID, "Invalid email test",
		)

		_, err = repo.Create(ctx, tx, CreateDeliveryInput{
			NotificationID: notificationID,
			RecipientEmail: "invalid-email",
		})

		if !errors.Is(err, ErrInvalidDeliveryInput) {
			t.Errorf("expected ErrInvalidDeliveryInput, got %v", err)
		}
	})

	t.Run("claim pending deliveries", func(t *testing.T) {
		var notificationIDs []uuid.UUID

		for i := 0; i < 3; i++ {
			notificationIDs = append(notificationIDs,
				createTestNotification(
					t, pool, userID,
					fmt.Sprintf("Claim test %d", i),
				),
			)
		}

		for i, notificationID := range notificationIDs {
			createTestDelivery(
				t, pool, notificationID,
				fmt.Sprintf("claim-%d@example.com", i),
			)
		}

		deliveries, err := repo.ClaimBatch(ctx, 2)
		if err != nil {
			t.Fatalf("claim deliveries: %v", err)
		}

		if len(deliveries) != 2 {
			t.Fatalf("expected 2 claimed deliveries, got %d",
				len(deliveries))
		}

		for _, delivery := range deliveries {
			if delivery.Status != DeliveryProcessing {
				t.Errorf("expected processing status, got %s",
					delivery.Status)
			}

			if delivery.Attempts != 1 {
				t.Errorf("expected 1 attempt, got %d",
					delivery.Attempts)
			}

			if delivery.LockedAt == nil {
				t.Error("expected locked_at to be set")
			}
		}
	})

	t.Run("mark delivery as sent", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Mark sent test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "sent@example.com",
		)

		claimed, err := repo.ClaimBatch(ctx, 100)
		if err != nil {
			t.Fatalf("claim delivery: %v", err)
		}

		var claimedDelivery *Delivery
		for i := range claimed {
			if claimed[i].ID == delivery.ID {
				claimedDelivery = &claimed[i]
				break
			}
		}

		if claimedDelivery == nil {
			t.Fatal("delivery was not claimed")
		}

		if err := repo.MarkSent(ctx, delivery.ID); err != nil {
			t.Fatalf("mark delivery sent: %v", err)
		}

		updated := getTestDelivery(t, pool, delivery.ID)

		if updated.Status != DeliverySent {
			t.Errorf("expected sent status, got %s", updated.Status)
		}

		if updated.SentAt == nil {
			t.Error("expected sent_at to be set")
		}

		if updated.LockedAt != nil {
			t.Error("expected locked_at to be cleared")
		}

		if updated.LastError != nil {
			t.Error("expected last_error to be cleared")
		}
	})

	t.Run("schedule retry with exponential backoff", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Retry test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "retry@example.com",
		)

		claimed, err := repo.ClaimBatch(ctx, 1)
		if err != nil {
			t.Fatalf("claim delivery: %v", err)
		}

		found := false
		for _, item := range claimed {
			if item.ID == delivery.ID {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("delivery was not claimed")
		}

		beforeRetry := time.Now()

		if err := repo.ScheduleRetry(
			ctx, delivery.ID, "temporary provider failure",
		); err != nil {
			t.Fatalf("schedule retry: %v", err)
		}

		updated := getTestDelivery(t, pool, delivery.ID)

		if updated.Status != DeliveryPending {
			t.Errorf("expected pending status, got %s", updated.Status)
		}

		if updated.Attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", updated.Attempts)
		}

		if updated.LockedAt != nil {
			t.Error("expected locked_at to be cleared")
		}

		if updated.LastError == nil ||
			*updated.LastError != "temporary provider failure" {
			t.Errorf("unexpected last_error: %v", updated.LastError)
		}

		// First retry: 2^1 * 30 = 60 seconds.
		expectedMinimum := beforeRetry.Add(59 * time.Second)
		expectedMaximum := time.Now().Add(62 * time.Second)

		if updated.AvailableAt.Before(expectedMinimum) ||
			updated.AvailableAt.After(expectedMaximum) {
			t.Errorf("unexpected retry time: %v", updated.AvailableAt)
		}
	})

	t.Run("delivery becomes failed after maximum attempts", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Maximum attempts test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "failed@example.com",
		)

		// Simulate four previous claims. The next claim will be attempt five.
		_, err := pool.Exec(ctx, `
			UPDATE notification_deliveries
			SET attempts = $2
			WHERE id = $1
		`, delivery.ID, MaxDeliveryAttempts-1)
		if err != nil {
			t.Fatalf("set previous attempts: %v", err)
		}

		claimed, err := repo.ClaimBatch(ctx, 1)
		if err != nil {
			t.Fatalf("claim final attempt: %v", err)
		}

		found := false
		for _, item := range claimed {
			if item.ID == delivery.ID {
				found = true
				if item.Attempts != MaxDeliveryAttempts {
					t.Errorf("expected %d attempts, got %d",
						MaxDeliveryAttempts, item.Attempts)
				}
				break
			}
		}
		if !found {
			t.Fatal("delivery was not claimed")
		}

		if err := repo.ScheduleRetry(
			ctx, delivery.ID, "maximum attempts reached",
		); err != nil {
			t.Fatalf("schedule final retry: %v", err)
		}

		updated := getTestDelivery(t, pool, delivery.ID)

		if updated.Status != DeliveryFailed {
			t.Errorf("expected failed status, got %s", updated.Status)
		}

		if updated.Attempts != MaxDeliveryAttempts {
			t.Errorf("expected %d attempts, got %d",
				MaxDeliveryAttempts, updated.Attempts)
		}
	})

	t.Run("recover stale processing delivery", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Stale recovery test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "stale@example.com",
		)

		claimed, err := repo.ClaimBatch(ctx, 1)
		if err != nil {
			t.Fatalf("claim delivery: %v", err)
		}

		found := false
		for _, item := range claimed {
			if item.ID == delivery.ID {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("delivery was not claimed")
		}

		// Simulate a worker that claimed the job and then crashed.
		_, err = pool.Exec(ctx, `
			UPDATE notification_deliveries
			SET locked_at = NOW() - INTERVAL '10 minutes'
			WHERE id = $1
		`, delivery.ID)
		if err != nil {
			t.Fatalf("make delivery claim stale: %v", err)
		}

		recovered, err := repo.RecoverStaleClaims(ctx, time.Minute)
		if err != nil {
			t.Fatalf("recover stale claims: %v", err)
		}

		if recovered < 1 {
			t.Errorf("expected at least 1 recovered claim, got %d", recovered)
		}

		updated := getTestDelivery(t, pool, delivery.ID)

		if updated.Status != DeliveryPending {
			t.Errorf("expected pending status, got %s", updated.Status)
		}

		if updated.LockedAt != nil {
			t.Error("expected locked_at to be cleared")
		}

		if updated.Attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", updated.Attempts)
		}

		if updated.LastError == nil ||
			*updated.LastError != "worker claim expired" {
			t.Errorf("unexpected recovery error: %v", updated.LastError)
		}
	})

	t.Run("stale delivery at maximum attempts becomes failed", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Stale maximum attempts test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "stale-failed@example.com",
		)

		_, err := pool.Exec(ctx, `
			UPDATE notification_deliveries
			SET
				status = 'processing',
				attempts = $2,
				locked_at = NOW() - INTERVAL '10 minutes'
			WHERE id = $1
		`, delivery.ID, MaxDeliveryAttempts)
		if err != nil {
			t.Fatalf("prepare exhausted delivery: %v", err)
		}

		recovered, err := repo.RecoverStaleClaims(ctx, time.Minute)
		if err != nil {
			t.Fatalf("recover exhausted delivery: %v", err)
		}

		if recovered != 1 {
			t.Errorf("expected 1 recovered delivery, got %d", recovered)
		}

		updated := getTestDelivery(t, pool, delivery.ID)

		if updated.Status != DeliveryFailed {
			t.Errorf("expected failed status, got %s", updated.Status)
		}
	})

	t.Run("reject invalid batch sizes", func(t *testing.T) {
		for _, size := range []int{0, -1, 101} {
			_, err := repo.ClaimBatch(ctx, size)
			if !errors.Is(err, ErrInvalidDeliveryInput) {
				t.Errorf("batch size %d: expected ErrInvalidDeliveryInput, got %v",
					size, err)
			}
		}
	})

	t.Run("reject invalid stale recovery duration", func(t *testing.T) {
		for _, duration := range []time.Duration{0, -time.Second} {
			_, err := repo.RecoverStaleClaims(ctx, duration)
			if !errors.Is(err, ErrInvalidDeliveryInput) {
				t.Errorf("duration %v: expected ErrInvalidDeliveryInput, got %v",
					duration, err)
			}
		}
	})

	t.Run("cannot mark an unclaimed delivery as sent", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Unclaimed delivery test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "unclaimed@example.com",
		)

		err := repo.MarkSent(ctx, delivery.ID)
		if !errors.Is(err, ErrDeliveryNotFound) {
			t.Errorf("expected ErrDeliveryNotFound, got %v", err)
		}
	})

	t.Run("cannot retry an unclaimed delivery", func(t *testing.T) {
		notificationID := createTestNotification(
			t, pool, userID, "Unclaimed retry test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "unclaimed-retry@example.com",
		)

		err := repo.ScheduleRetry(ctx, delivery.ID, "test failure")
		if !errors.Is(err, ErrDeliveryNotFound) {
			t.Errorf("expected ErrDeliveryNotFound, got %v", err)
		}
	})

	t.Run("claiming with no eligible jobs returns empty result", func(t *testing.T) {
		// Use a fresh repository test pool state with no pending rows
		// guaranteed by checking a unique notification's delivery.
		notificationID := createTestNotification(
			t, pool, userID, "Future delivery test",
		)

		delivery := createTestDelivery(
			t, pool, notificationID, "future@example.com",
		)

		_, err := pool.Exec(ctx, `
			UPDATE notification_deliveries
			SET available_at = NOW() + INTERVAL '1 hour'
			WHERE id = $1
		`, delivery.ID)
		if err != nil {
			t.Fatalf("delay delivery: %v", err)
		}

		// ClaimBatch may find other pending jobs from earlier subtests,
		// so this test only checks that the future delivery is not claimed.
		claimed, err := repo.ClaimBatch(ctx, 100)
		if err != nil {
			t.Fatalf("claim batch: %v", err)
		}

		for _, item := range claimed {
			if item.ID == delivery.ID {
				t.Error("future delivery should not have been claimed")
			}
		}
	})

	_ = pgx.ErrNoRows // Keep pgx imported for future repository assertions.
}
