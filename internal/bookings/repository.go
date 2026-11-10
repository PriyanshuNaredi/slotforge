package bookings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/priyanshunaredi/slotforge/internal/notifications"
)

var (
	ErrSlotNotFound          = errors.New("availability slot not found")
	ErrSlotClosed            = errors.New("availability slot is closed")
	ErrSlotBooked            = errors.New("availability slot is already booked")
	ErrSlotBlocked           = errors.New("availability slot falls in an unavailable period")
	ErrBookingNotFound       = errors.New("booking not found")
	ErrBookingNotCancellable = errors.New("booking cannot be cancelled")
	// Reschedule-specific rejections.
	ErrBookingNotReschedulable = errors.New("booking cannot be rescheduled")
	ErrSlotCrossProvider       = errors.New("target slot belongs to a different provider")
)

type Repository struct {
	db            *pgxpool.Pool
	notifications *notifications.Repository
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:            db,
		notifications: notifications.NewRepository(db),
	}
}
func (r *Repository) notificationRepository() *notifications.Repository {
	if r.notifications != nil {
		return r.notifications
	}

	return notifications.NewRepository(r.db)
}

func (r *Repository) Create(
	ctx context.Context,
	customerID uuid.UUID,
	input CreateBookingInput,
) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin booking transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var (
		slotStatus string
		providerID uuid.UUID
		startsAt   time.Time
		endsAt     time.Time
	)

	err = tx.QueryRow(ctx, `
	SELECT status, provider_id, starts_at, ends_at
	FROM availability_slots
	WHERE id = $1
	FOR UPDATE
`, input.SlotID).Scan(&slotStatus, &providerID, &startsAt, &endsAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSlotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock availability slot: %w", err)
	}

	if slotStatus != "open" {
		return nil, ErrSlotClosed
	}

	// A blocked provider window may not be booked.
	var blocked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM provider_unavailable_periods
			WHERE provider_id = $1
			  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)')
		)
	`, providerID, startsAt, endsAt).Scan(&blocked); err != nil {
		return nil, fmt.Errorf("check unavailable period: %w", err)
	}
	if blocked {
		return nil, ErrSlotBlocked
	}

	var alreadyBooked bool

	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE slot_id = $1
			  AND status IN ('pending', 'confirmed')
		)
	`, input.SlotID).Scan(&alreadyBooked)

	if err != nil {
		return nil, fmt.Errorf("check existing booking: %w", err)
	}

	if alreadyBooked {
		return nil, ErrSlotBooked
	}

	var booking Booking

	err = tx.QueryRow(ctx, `
		INSERT INTO bookings (
			slot_id,
			customer_id,
			status,
			notes
		)
		VALUES ($1, $2, 'confirmed', $3)
		RETURNING
			id,
			slot_id,
			customer_id,
			status,
			notes,
			created_at,
			updated_at
	`,
		input.SlotID,
		customerID,
		input.Notes,
	).Scan(
		&booking.ID,
		&booking.SlotID,
		&booking.CustomerID,
		&booking.Status,
		&booking.Notes,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "bookings_one_active_per_slot" {
			return nil, ErrSlotBooked
		}

		return nil, fmt.Errorf("insert booking: %w", err)
	}

	// Create the customer's booking confirmation notification.
	_, _, err = r.notificationRepository().CreateWithDelivery(ctx, tx, notifications.CreateInput{
		UserID:    customerID,
		BookingID: &booking.ID,
		Type:      "booking_confirmed",
		Title:     "Booking confirmed",
		Message:   "Your booking has been confirmed successfully.",
	})

	if err != nil {
		return nil, fmt.Errorf("create customer booking notification: %w", err)
	}

	// Notify the provider about the new booking.
	_, _, err = r.notificationRepository().CreateWithDelivery(ctx, tx, notifications.CreateInput{
		UserID:    providerID,
		BookingID: &booking.ID,
		Type:      "new_booking",
		Title:     "New booking received",
		Message:   "A customer has booked one of your available slots.",
	})

	if err != nil {
		return nil, fmt.Errorf("create provider booking notification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit booking transaction: %w", err)
	}

	return &booking, nil
}

// GetByIDForUser returns full booking detail for authorized viewers only:
// the booking's customer, the provider who owns the booked slot, or an
// administrator. Everyone else — other customers, other providers — gets
// pgx.ErrNoRows, indistinguishable from a missing booking. The slot join has
// no status or time filter on purpose: slots are never deleted (FK ON DELETE
// RESTRICT), so closed and past slots still resolve and historical bookings
// stay fully displayable.
func (r *Repository) GetByIDForUser(
	ctx context.Context,
	id uuid.UUID,
	callerID uuid.UUID,
	isAdmin bool,
) (*BookingDetail, error) {
	var detail BookingDetail

	err := r.db.QueryRow(ctx, `
		SELECT
			b.id,
			b.slot_id,
			b.customer_id,
			b.status,
			b.notes,
			b.created_at,
			b.updated_at,
			s.status,
			s.starts_at,
			s.ends_at,
			s.provider_id,
			p.display_name,
			p.timezone,
			u.email
		FROM bookings b
		JOIN availability_slots s
			ON s.id = b.slot_id
		JOIN provider_profiles p
			ON p.user_id = s.provider_id
		JOIN users u
			ON u.id = b.customer_id
		WHERE b.id = $1
		  AND ($2::boolean OR b.customer_id = $3 OR s.provider_id = $3)
	`, id, isAdmin, callerID).Scan(
		&detail.ID,
		&detail.SlotID,
		&detail.CustomerID,
		&detail.Status,
		&detail.Notes,
		&detail.CreatedAt,
		&detail.UpdatedAt,
		&detail.SlotStatus,
		&detail.StartsAt,
		&detail.EndsAt,
		&detail.ProviderID,
		&detail.ProviderName,
		&detail.ProviderTimezone,
		&detail.CustomerEmail,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("get booking detail: %w", err)
	}

	return &detail, nil
}

func (r *Repository) SearchAvailableSlots(
	ctx context.Context,
	input SlotSearchInput,
) ([]AvailableSlot, error) {

	query := `
		SELECT
			s.id,
			s.provider_id,
			p.display_name,
			p.timezone,
			s.starts_at,
			s.ends_at
		FROM availability_slots s
		JOIN provider_profiles p
			ON p.user_id = s.provider_id
		WHERE s.status = 'open'
		  AND s.starts_at > NOW()
		  AND NOT EXISTS (
    SELECT 1
    FROM bookings b
    WHERE b.slot_id = s.id
      AND b.status IN ('pending', 'confirmed')
)
		  AND NOT EXISTS (
			SELECT 1
			FROM provider_unavailable_periods up
			WHERE up.provider_id = s.provider_id
			  AND tstzrange(up.starts_at, up.ends_at, '[)') && tstzrange(s.starts_at, s.ends_at, '[)')
)
			  AND (
				  $1::uuid IS NULL
				  OR s.provider_id = $1
			  )
			  AND (
				  $2::timestamptz IS NULL
				  OR s.starts_at >= $2
			  )
			  AND (
				  $3::timestamptz IS NULL
				  OR s.starts_at < $3
			  )
			ORDER BY s.starts_at ASC
			LIMIT $4
			OFFSET $5
		`

	rows, err := r.db.Query(
		ctx,
		query,
		input.ProviderID,
		input.From,
		input.To,
		input.Limit,
		input.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("search available slots: %w", err)
	}
	defer rows.Close()

	slots := make([]AvailableSlot, 0)

	for rows.Next() {
		var slot AvailableSlot

		err := rows.Scan(
			&slot.ID,
			&slot.ProviderID,
			&slot.DisplayName,
			&slot.Timezone,
			&slot.StartsAt,
			&slot.EndsAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan available slot: %w", err)
		}

		slots = append(slots, slot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate available slots: %w", err)
	}

	return slots, nil
}

func (r *Repository) CountAvailableSlots(
	ctx context.Context,
	input SlotSearchInput,
) (int64, error) {

	query := `
        SELECT COUNT(*)
        FROM availability_slots s
        WHERE s.status = 'open'
          AND s.starts_at > NOW()
          AND NOT EXISTS (
              SELECT 1
              FROM bookings b
              WHERE b.slot_id = s.id
                AND b.status IN ('pending', 'confirmed')
          )
          AND NOT EXISTS (
              SELECT 1
              FROM provider_unavailable_periods up
              WHERE up.provider_id = s.provider_id
                AND tstzrange(up.starts_at, up.ends_at, '[)') && tstzrange(s.starts_at, s.ends_at, '[)')
          )
          AND ($1::uuid IS NULL OR s.provider_id = $1)
          AND ($2::timestamptz IS NULL OR s.starts_at >= $2)
          AND ($3::timestamptz IS NULL OR s.starts_at < $3)
    `

	var total int64

	err := r.db.QueryRow(
		ctx,
		query,
		input.ProviderID,
		input.From,
		input.To,
	).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("count available slots: %w", err)
	}

	return total, nil
}

func (r *Repository) GetAvailableSlotByID(
	ctx context.Context,
	slotID uuid.UUID,
) (*AvailableSlot, error) {

	query := `
		SELECT
			s.id,
			s.provider_id,
			p.display_name,
			p.timezone,
			s.starts_at,
			s.ends_at
		FROM availability_slots s
		JOIN provider_profiles p
			ON p.user_id = s.provider_id
		WHERE s.id = $1
		  AND s.status = 'open'
		  AND s.starts_at > NOW()
		  AND NOT EXISTS (
			SELECT 1
			FROM provider_unavailable_periods up
			WHERE up.provider_id = s.provider_id
			  AND tstzrange(up.starts_at, up.ends_at, '[)') && tstzrange(s.starts_at, s.ends_at, '[)')
		  )
	`

	var slot AvailableSlot

	err := r.db.QueryRow(ctx, query, slotID).Scan(
		&slot.ID,
		&slot.ProviderID,
		&slot.DisplayName,
		&slot.Timezone,
		&slot.StartsAt,
		&slot.EndsAt,
	)

	if err != nil {
		return nil, err
	}

	return &slot, nil
}

func (r *Repository) ListByCustomer(
	ctx context.Context,
	customerID uuid.UUID,
	input BookingListInput,
) ([]Booking, error) {

	query := `
		SELECT
			id,
			slot_id,
			customer_id,
			status,
			notes,
			created_at,
			updated_at
		FROM bookings
		WHERE customer_id = $1
		  AND ($2::varchar IS NULL OR status = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3
		OFFSET $4
	`

	rows, err := r.db.Query(
		ctx,
		query,
		customerID,
		input.Status,
		input.Limit,
		input.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list customer bookings: %w", err)
	}
	defer rows.Close()

	bookings := make([]Booking, 0)

	for rows.Next() {
		var booking Booking

		err := rows.Scan(
			&booking.ID,
			&booking.SlotID,
			&booking.CustomerID,
			&booking.Status,
			&booking.Notes,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan customer booking: %w", err)
		}

		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate customer bookings: %w", err)
	}

	return bookings, nil
}

func (r *Repository) CountByCustomer(
	ctx context.Context,
	customerID uuid.UUID,
	input BookingListInput,
) (int64, error) {

	query := `
		SELECT COUNT(*)
		FROM bookings
		WHERE customer_id = $1
		  AND ($2::varchar IS NULL OR status = $2)
	`

	var total int64

	err := r.db.QueryRow(
		ctx,
		query,
		customerID,
		input.Status,
	).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("count customer bookings: %w", err)
	}

	return total, nil
}

func (r *Repository) CancelByCustomer(
	ctx context.Context,
	bookingID uuid.UUID,
	customerID uuid.UUID,
) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin cancellation transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// First, find the slot associated with this customer's booking.
	// This initial read does not lock the booking row.
	var slotID uuid.UUID

	err = tx.QueryRow(ctx, `
		SELECT slot_id
		FROM bookings
		WHERE id = $1
		  AND customer_id = $2
	`, bookingID, customerID).Scan(&slotID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find booking slot: %w", err)
	}

	// Lock the slot first. This preserves the same lock order
	// used by booking creation and provider slot closure.
	var (
		lockedSlotID uuid.UUID
		providerID   uuid.UUID
	)

	err = tx.QueryRow(ctx, `
    SELECT id, provider_id
    FROM availability_slots
    WHERE id = $1
    FOR UPDATE
`, slotID).Scan(&lockedSlotID, &providerID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSlotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock availability slot: %w", err)
	}

	// Lock and re-check the booking after acquiring the slot lock.
	var booking Booking

	err = tx.QueryRow(ctx, `
		SELECT
			id,
			slot_id,
			customer_id,
			status,
			notes,
			created_at,
			updated_at
		FROM bookings
		WHERE id = $1
		  AND customer_id = $2
		FOR UPDATE
	`, bookingID, customerID).Scan(
		&booking.ID,
		&booking.SlotID,
		&booking.CustomerID,
		&booking.Status,
		&booking.Notes,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock booking: %w", err)
	}

	if booking.Status != StatusConfirmed {
		return nil, ErrBookingNotCancellable
	}

	// Only confirmed bookings can be cancelled.
	err = tx.QueryRow(ctx, `
		UPDATE bookings
		SET status = 'cancelled',
		    updated_at = NOW()
		WHERE id = $1
		  AND customer_id = $2
		RETURNING updated_at
	`, bookingID, customerID).Scan(&booking.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("cancel booking: %w", err)
	}

	booking.Status = StatusCancelled
	// Notify the customer about the cancellation.
	_, _, err = r.notificationRepository().CreateWithDelivery(ctx, tx, notifications.CreateInput{
		UserID:    customerID,
		BookingID: &booking.ID,
		Type:      "booking_cancelled",
		Title:     "Booking cancelled",
		Message:   "Your booking has been cancelled successfully.",
	})
	if err != nil {
		return nil, fmt.Errorf("create customer cancellation notification: %w", err)
	}

	// Notify the provider about the cancellation.
	_, _, err = r.notificationRepository().CreateWithDelivery(ctx, tx, notifications.CreateInput{
		UserID:    providerID,
		BookingID: &booking.ID,
		Type:      "customer_cancelled",
		Title:     "Customer cancelled booking",
		Message:   "A customer has cancelled their booking.",
	})
	if err != nil {
		return nil, fmt.Errorf("create provider cancellation notification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancellation transaction: %w", err)
	}

	return &booking, nil
}

// RescheduleByCustomer moves a confirmed booking to a new slot of the same
// provider in one transaction. Lock order matches booking creation and
// cancellation — target slot first, then the booking row — so competing
// reschedules into the same slot serialize instead of deadlocking. The old
// slot needs no lock: slot availability is derived from active bookings, so
// the UPDATE alone frees it, and the partial unique index
// bookings_one_active_per_slot backstops any race the check below misses.
// Rescheduling onto the booking's current slot is an idempotent no-op.
func (r *Repository) RescheduleByCustomer(
	ctx context.Context,
	bookingID uuid.UUID,
	customerID uuid.UUID,
	newSlotID uuid.UUID,
) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin reschedule transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Lock the target slot first, mirroring the booking-creation lock order.
	var (
		newSlotProvider uuid.UUID
		newSlotStatus   string
	)

	err = tx.QueryRow(ctx, `
		SELECT provider_id, status
		FROM availability_slots
		WHERE id = $1
		FOR UPDATE
	`, newSlotID).Scan(&newSlotProvider, &newSlotStatus)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSlotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock target slot: %w", err)
	}

	// Lock the booking under the slot lock and validate ownership in the
	// same query: another customer's booking looks like a missing one.
	var booking Booking

	err = tx.QueryRow(ctx, `
		SELECT
			id,
			slot_id,
			customer_id,
			status,
			notes,
			created_at,
			updated_at
		FROM bookings
		WHERE id = $1
		  AND customer_id = $2
		FOR UPDATE
	`, bookingID, customerID).Scan(
		&booking.ID,
		&booking.SlotID,
		&booking.CustomerID,
		&booking.Status,
		&booking.Notes,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock booking: %w", err)
	}

	// Only confirmed appointments can move; completed, cancelled, and
	// no-show bookings are history.
	if booking.Status != StatusConfirmed {
		return nil, ErrBookingNotReschedulable
	}

	// Rescheduling onto the current slot changes nothing: succeed without
	// touching the row or creating notifications (idempotent retries).
	if booking.SlotID == newSlotID {
		return &booking, nil
	}

	if newSlotStatus != "open" {
		return nil, ErrSlotClosed
	}

	// The new slot must belong to the same provider as the existing one.
	var oldSlotProvider uuid.UUID

	err = tx.QueryRow(ctx, `
		SELECT provider_id
		FROM availability_slots
		WHERE id = $1
	`, booking.SlotID).Scan(&oldSlotProvider)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSlotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load current slot: %w", err)
	}

	if oldSlotProvider != newSlotProvider {
		return nil, ErrSlotCrossProvider
	}

	// The target slot must not carry another active booking.
	var alreadyBooked bool

	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE slot_id = $1
			  AND status IN ('pending', 'confirmed')
			  AND id <> $2
		)
	`, newSlotID, booking.ID).Scan(&alreadyBooked)

	if err != nil {
		return nil, fmt.Errorf("check target slot booking: %w", err)
	}

	if alreadyBooked {
		return nil, ErrSlotBooked
	}

	// The target slot may not sit inside a blocked provider window.
	var rescheduleBlocked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM availability_slots s
			JOIN provider_unavailable_periods up ON up.provider_id = s.provider_id
			WHERE s.id = $1
			  AND tstzrange(up.starts_at, up.ends_at, '[)') && tstzrange(s.starts_at, s.ends_at, '[)')
		)
	`, newSlotID).Scan(&rescheduleBlocked); err != nil {
		return nil, fmt.Errorf("check target slot unavailability: %w", err)
	}
	if rescheduleBlocked {
		return nil, ErrSlotBlocked
	}

	// The move itself: same booking ID, same history, new slot.
	err = tx.QueryRow(ctx, `
		UPDATE bookings
		SET slot_id = $2,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`, booking.ID, newSlotID).Scan(&booking.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "bookings_one_active_per_slot" {
			return nil, ErrSlotBooked
		}

		return nil, fmt.Errorf("reschedule booking: %w", err)
	}

	booking.SlotID = newSlotID

	// Notify the customer about the reschedule.
	_, _, err = r.notificationRepository().CreateWithDelivery(ctx, tx, notifications.CreateInput{
		UserID:    customerID,
		BookingID: &booking.ID,
		Type:      "booking_rescheduled",
		Title:     "Appointment rescheduled",
		Message:   "Your appointment has been moved to a new time.",
	})

	if err != nil {
		return nil, fmt.Errorf("create customer reschedule notification: %w", err)
	}

	// Notify the provider about the reschedule.
	_, _, err = r.notificationRepository().CreateWithDelivery(ctx, tx, notifications.CreateInput{
		UserID:    newSlotProvider,
		BookingID: &booking.ID,
		Type:      "customer_rescheduled",
		Title:     "Appointment rescheduled by customer",
		Message:   "A customer has moved their booking to a different slot.",
	})

	if err != nil {
		return nil, fmt.Errorf("create provider reschedule notification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reschedule transaction: %w", err)
	}

	return &booking, nil
}
