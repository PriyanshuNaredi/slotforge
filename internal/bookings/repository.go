package bookings

import (
	"context"
	"errors"
	"fmt"

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
	ErrBookingNotFound       = errors.New("booking not found")
	ErrBookingNotCancellable = errors.New("booking cannot be cancelled")
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
	)

	err = tx.QueryRow(ctx, `
	SELECT status, provider_id
	FROM availability_slots
	WHERE id = $1
	FOR UPDATE
`, input.SlotID).Scan(&slotStatus, &providerID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSlotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock availability slot: %w", err)
	}

	if slotStatus != "open" {
		return nil, ErrSlotClosed
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

// GetByIDForUser hides bookings belonging to other customers.
// Administrators can retrieve any booking.
func (r *Repository) GetByIDForUser(
	ctx context.Context,
	id uuid.UUID,
	customerID uuid.UUID,
	isAdmin bool,
) (*Booking, error) {
	var booking Booking

	err := r.db.QueryRow(ctx, `
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
		  AND ($2::boolean OR customer_id = $3)
	`, id, isAdmin, customerID).Scan(
		&booking.ID,
		&booking.SlotID,
		&booking.CustomerID,
		&booking.Status,
		&booking.Notes,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("get booking: %w", err)
	}

	return &booking, nil
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
		ORDER BY created_at DESC
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
