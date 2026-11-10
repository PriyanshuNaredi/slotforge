package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshunaredi/slotforge/internal/notifications"
)

var (
	ErrProfileExists   = errors.New("provider profile already exists")
	ErrProfileNotFound = errors.New("provider profile not found")

	ErrProviderBookingNotFound  = errors.New("provider booking not found")
	ErrInvalidBookingTransition = errors.New("invalid booking status transition")
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

func canTransitionBookingStatus(current, next string) bool {
	switch current {
	case "pending":
		return next == "confirmed" || next == "cancelled"

	case "confirmed":
		return next == "cancelled" ||
			next == "completed" ||
			next == "no_show"

	default:
		// cancelled, completed, and no_show are terminal states.
		return false
	}
}

func (r *Repository) Create(
	ctx context.Context,
	userID uuid.UUID,
	input CreateProfileInput,
) (*Profile, error) {
	var profile Profile

	err := r.db.QueryRow(ctx, `
		INSERT INTO provider_profiles (
			user_id,
			display_name,
			bio,
			timezone
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			user_id,
			display_name,
			bio,
			timezone,
			created_at,
			updated_at
	`,
		userID,
		input.DisplayName,
		input.Bio,
		input.Timezone,
	).Scan(
		&profile.UserID,
		&profile.DisplayName,
		&profile.Bio,
		&profile.Timezone,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrProfileExists
		}

		return nil, fmt.Errorf("create provider profile: %w", err)
	}

	return &profile, nil
}

func (r *Repository) GetByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*Profile, error) {
	var profile Profile

	err := r.db.QueryRow(ctx, `
		SELECT
			user_id,
			display_name,
			bio,
			timezone,
			created_at,
			updated_at
		FROM provider_profiles
		WHERE user_id = $1
	`, userID).Scan(
		&profile.UserID,
		&profile.DisplayName,
		&profile.Bio,
		&profile.Timezone,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provider profile: %w", err)
	}

	return &profile, nil
}

func (r *Repository) CreateSlot(
	ctx context.Context,
	providerID uuid.UUID,
	input CreateSlotInput,
) (*Slot, error) {
	if !input.EndsAt.After(input.StartsAt) {
		return nil, ErrInvalidSlotTime
	}

	// New availability may not overlap a blocked period. The guard lives in
	// the statement itself so a period committed mid-flight cannot be missed.
	query := `
		INSERT INTO availability_slots (
			provider_id,
			starts_at,
			ends_at
		)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM provider_unavailable_periods
			WHERE provider_id = $1
			  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)')
		)
		RETURNING
			id,
			provider_id,
			starts_at,
			ends_at,
			status,
			created_at,
			updated_at
	`

	var slot Slot

	err := r.db.QueryRow(
		ctx,
		query,
		providerID,
		input.StartsAt,
		input.EndsAt,
	).Scan(
		&slot.ID,
		&slot.ProviderID,
		&slot.StartsAt,
		&slot.EndsAt,
		&slot.Status,
		&slot.CreatedAt,
		&slot.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return nil, ErrSlotOverlap
		}

		if errors.Is(err, pgx.ErrNoRows) {
			// The INSERT..SELECT guard skipped the row: a blocked period overlaps.
			return nil, ErrSlotBlockedByUnavailability
		}

		return nil, fmt.Errorf("create availability slot: %w", err)
	}

	return &slot, nil
}

// mapTransientTxError reports PostgreSQL deadlock (40P01) and serialization
// (40001) failures as a retryable sentinel instead of an opaque 500.
func mapTransientTxError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40P01" || pgErr.Code == "40001") {
		return ErrUnavailableLocked
	}
	return nil
}

// CreateUnavailablePeriod blocks a window. Overlapping an ACTIVE booking
// (pending/confirmed) is rejected inside the same transaction, and the
// overlapping slot rows are locked FOR UPDATE first: booking creation and
// rescheduling take the same slot lock before checking availability, so a
// booking can never commit into a window that this transaction blocks.
func (r *Repository) CreateUnavailablePeriod(
	ctx context.Context,
	providerID uuid.UUID,
	input CreateUnavailablePeriodInput,
) (*UnavailablePeriod, error) {
	if !input.EndsAt.After(input.StartsAt) {
		return nil, ErrInvalidUnavailability
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin unavailable period transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Serialize against booking creation/rescheduling, which lock the target
	// slot first: without this shared lock both sides could check "nothing
	// conflicts" and commit a booking inside a blocked window. Rows are
	// locked in deterministic id order so two concurrent blocks on
	// overlapping windows cannot deadlock waiting for each other's rows.
	rows, err := tx.Query(ctx, `
		SELECT id FROM availability_slots
		WHERE provider_id = $1
		  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)')
		ORDER BY id
		FOR UPDATE
	`, providerID, input.StartsAt, input.EndsAt)
	if err != nil {
		if transient := mapTransientTxError(err); transient != nil {
			return nil, transient
		}
		return nil, fmt.Errorf("lock overlapping slots: %w", err)
	}
	rows.Close()

	var hasBookings bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM bookings b
			JOIN availability_slots s ON s.id = b.slot_id
			WHERE s.provider_id = $1
			  AND b.status IN ('pending', 'confirmed')
			  AND tstzrange(s.starts_at, s.ends_at, '[)')
			      && tstzrange($2, $3, '[)')
		)
	`, providerID, input.StartsAt, input.EndsAt).Scan(&hasBookings)
	if err != nil {
		return nil, fmt.Errorf("check active bookings in window: %w", err)
	}
	if hasBookings {
		return nil, ErrUnavailableHasBookings
	}

	var period UnavailablePeriod

	err = tx.QueryRow(ctx, `
		INSERT INTO provider_unavailable_periods (provider_id, starts_at, ends_at, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id, provider_id, starts_at, ends_at, reason, created_at
	`,
		providerID, input.StartsAt, input.EndsAt, input.Reason,
	).Scan(
		&period.ID, &period.ProviderID, &period.StartsAt, &period.EndsAt,
		&period.Reason, &period.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create unavailable period: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		if transient := mapTransientTxError(err); transient != nil {
			return nil, transient
		}
		return nil, fmt.Errorf("commit unavailable period: %w", err)
	}

	return &period, nil
}

// ListUnavailablePeriods returns the provider's blocked windows, soonest first.
func (r *Repository) ListUnavailablePeriods(
	ctx context.Context,
	providerID uuid.UUID,
) ([]UnavailablePeriod, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, provider_id, starts_at, ends_at, reason, created_at
		FROM provider_unavailable_periods
		WHERE provider_id = $1
		ORDER BY starts_at
	`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list unavailable periods: %w", err)
	}
	defer rows.Close()

	periods := make([]UnavailablePeriod, 0)
	for rows.Next() {
		var p UnavailablePeriod
		if err := rows.Scan(
			&p.ID, &p.ProviderID, &p.StartsAt, &p.EndsAt, &p.Reason, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan unavailable period: %w", err)
		}
		periods = append(periods, p)
	}

	return periods, rows.Err()
}

// DeleteUnavailablePeriod removes a blocked window owned by the provider.
func (r *Repository) DeleteUnavailablePeriod(
	ctx context.Context,
	providerID, periodID uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM provider_unavailable_periods
		WHERE id = $1 AND provider_id = $2
	`, periodID, providerID)
	if err != nil {
		return fmt.Errorf("delete unavailable period: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrUnavailableNotFound
	}

	return nil
}

// CreateSlotsBatch inserts concrete slot windows in one transaction. Exact
// duplicates (same provider and start instant) are skipped so re-running a
// rule is idempotent; an overlap with a different open slot trips the
// availability_no_overlap exclusion constraint and rolls the whole batch
// back (ErrSlotOverlap).
func (r *Repository) CreateSlotsBatch(
	ctx context.Context,
	providerID uuid.UUID,
	slots []SlotTime,
) (created int64, skipped int64, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin slot batch: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const insert = `
		INSERT INTO availability_slots (provider_id, starts_at, ends_at)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM availability_slots
			WHERE provider_id = $1 AND starts_at = $2
		)
		  AND NOT EXISTS (
			SELECT 1 FROM provider_unavailable_periods
			WHERE provider_id = $1
			  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)')
		)
	`

	for _, s := range slots {
		tag, err := tx.Exec(ctx, insert, providerID, s.StartsAt, s.EndsAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
				return 0, 0, ErrSlotOverlap
			}
			return 0, 0, fmt.Errorf("insert recurring slot: %w", err)
		}
		if tag.RowsAffected() == 1 {
			created++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit slot batch: %w", err)
	}

	return created, skipped, nil
}

func (r *Repository) ListSlots(
	ctx context.Context,
	providerID uuid.UUID,
) ([]Slot, error) {
	query := `
		SELECT
			id,
			provider_id,
			starts_at,
			ends_at,
			status,
			created_at,
			updated_at
		FROM availability_slots
		WHERE provider_id = $1
		ORDER BY starts_at ASC
	`

	rows, err := r.db.Query(ctx, query, providerID)
	if err != nil {
		return nil, fmt.Errorf("list availability slots: %w", err)
	}
	defer rows.Close()

	slots := make([]Slot, 0)

	for rows.Next() {
		var slot Slot

		err := rows.Scan(
			&slot.ID,
			&slot.ProviderID,
			&slot.StartsAt,
			&slot.EndsAt,
			&slot.Status,
			&slot.CreatedAt,
			&slot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan availability slot: %w", err)
		}

		slots = append(slots, slot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate availability slots: %w", err)
	}

	return slots, nil
}

func (r *Repository) GetSlot(
	ctx context.Context,
	providerID uuid.UUID,
	slotID uuid.UUID,
) (*Slot, error) {
	query := `
		SELECT
			id,
			provider_id,
			starts_at,
			ends_at,
			status,
			created_at,
			updated_at
		FROM availability_slots
		WHERE provider_id = $1
		  AND id = $2
	`

	var slot Slot

	err := r.db.QueryRow(
		ctx,
		query,
		providerID,
		slotID,
	).Scan(
		&slot.ID,
		&slot.ProviderID,
		&slot.StartsAt,
		&slot.EndsAt,
		&slot.Status,
		&slot.CreatedAt,
		&slot.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSlotNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get availability slot: %w", err)
	}

	return &slot, nil
}

func (r *Repository) CloseSlot(
	ctx context.Context,
	providerID uuid.UUID,
	slotID uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin close slot transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var status string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM availability_slots
		WHERE id = $1 AND provider_id = $2
		FOR UPDATE
	`, slotID, providerID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSlotNotFound
	}

	if err != nil {
		return fmt.Errorf("lock availability slot: %w", err)
	}

	if status == SlotStatusClosed {
		return tx.Commit(ctx)
	}

	var hasActiveBooking bool

	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE slot_id = $1
			  AND status IN ('pending', 'confirmed')
		)
	`, slotID).Scan(&hasActiveBooking)

	if err != nil {
		return fmt.Errorf("check active bookings: %w", err)
	}

	if hasActiveBooking {
		return ErrSlotHasActiveBooking
	}

	_, err = tx.Exec(ctx, `
		UPDATE availability_slots
		SET status = 'closed',
		    updated_at = NOW()
		WHERE id = $1 AND provider_id = $2
	`, slotID, providerID)

	if err != nil {
		return fmt.Errorf("close availability slot: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit close slot transaction: %w", err)
	}

	return nil
}

func (r *Repository) ListBookings(
	ctx context.Context,
	providerID uuid.UUID,
	input ProviderBookingListInput,
) ([]ProviderBooking, error) {

	query := `
		SELECT
			b.id,
			b.slot_id,
			b.customer_id,
			u.email,
			b.status,
			b.notes,
			s.starts_at,
			s.ends_at,
			b.created_at,
			b.updated_at
		FROM bookings b
		JOIN availability_slots s
			ON s.id = b.slot_id
		JOIN users u
			ON u.id = b.customer_id
		WHERE s.provider_id = $1
		  AND ($2::varchar IS NULL OR b.status = $2)
		  AND ($3::timestamptz IS NULL OR s.starts_at >= $3)
		  AND ($4::timestamptz IS NULL OR s.starts_at < $4)
		ORDER BY s.starts_at DESC, b.created_at DESC, b.id DESC
		LIMIT $5
		OFFSET $6
	`

	var statusFilter any
	if input.Status != "" {
		statusFilter = input.Status
	}

	rows, err := r.db.Query(
		ctx,
		query,
		providerID,
		statusFilter,
		input.From,
		input.To,
		input.Limit,
		input.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider bookings: %w", err)
	}
	defer rows.Close()

	bookings := make([]ProviderBooking, 0)

	for rows.Next() {
		var booking ProviderBooking

		err := rows.Scan(
			&booking.ID,
			&booking.SlotID,
			&booking.CustomerID,
			&booking.CustomerEmail,
			&booking.Status,
			&booking.Notes,
			&booking.SlotStartsAt,
			&booking.SlotEndsAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan provider booking: %w", err)
		}

		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider bookings: %w", err)
	}

	return bookings, nil
}

func (r *Repository) CountBookings(
	ctx context.Context,
	providerID uuid.UUID,
	input ProviderBookingListInput,
) (int64, error) {

	query := `
		SELECT COUNT(*)
		FROM bookings b
		JOIN availability_slots s
			ON s.id = b.slot_id
		WHERE s.provider_id = $1
		  AND ($2::varchar IS NULL OR b.status = $2)
		  AND ($3::timestamptz IS NULL OR s.starts_at >= $3)
		  AND ($4::timestamptz IS NULL OR s.starts_at < $4)
	`

	var statusFilter any
	if input.Status != "" {
		statusFilter = input.Status
	}

	var total int64

	err := r.db.QueryRow(
		ctx,
		query,
		providerID,
		statusFilter,
		input.From,
		input.To,
	).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("count provider bookings: %w", err)
	}

	return total, nil
}

func (r *Repository) UpdateBookingStatus(
	ctx context.Context,
	providerID uuid.UUID,
	bookingID uuid.UUID,
	newStatus string,
) (*ProviderBooking, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin booking status transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Find the slot belonging to this provider.
	// This initial lookup does not lock the booking.
	var slotID uuid.UUID

	err = tx.QueryRow(ctx, `
		SELECT b.slot_id
		FROM bookings b
		JOIN availability_slots s ON s.id = b.slot_id
		WHERE b.id = $1
		  AND s.provider_id = $2
	`, bookingID, providerID).Scan(&slotID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProviderBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find provider booking: %w", err)
	}

	// 2. Lock the availability slot first.
	var lockedSlotID uuid.UUID

	err = tx.QueryRow(ctx, `
		SELECT id
		FROM availability_slots
		WHERE id = $1
		  AND provider_id = $2
		FOR UPDATE
	`, slotID, providerID).Scan(&lockedSlotID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProviderBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock availability slot: %w", err)
	}

	// 3. Lock the booking second.
	var (
		currentStatus string
		customerID    uuid.UUID
	)

	err = tx.QueryRow(ctx, `
	SELECT status, customer_id
	FROM bookings
	WHERE id = $1
	  AND slot_id = $2
	FOR UPDATE
`, bookingID, lockedSlotID).Scan(&currentStatus, &customerID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProviderBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock provider booking: %w", err)
	}

	// 4. Validate the state transition.
	if !canTransitionBookingStatus(currentStatus, newStatus) {
		return nil, ErrInvalidBookingTransition
	}

	// 5. Update the booking.
	_, err = tx.Exec(ctx, `
		UPDATE bookings
		SET status = $1,
		    updated_at = NOW()
		WHERE id = $2
	`, newStatus, bookingID)

	if err != nil {
		return nil, fmt.Errorf("update provider booking status: %w", err)
	}

	// Determine the notification based on the new booking status.
	var (
		notificationType    string
		notificationTitle   string
		notificationMessage string
	)

	switch newStatus {
	case "confirmed":
		notificationType = "booking_confirmed"
		notificationTitle = "Booking confirmed"
		notificationMessage = "Your booking has been confirmed by the provider."

	case "cancelled":
		notificationType = "booking_cancelled_by_provider"
		notificationTitle = "Booking cancelled"
		notificationMessage = "The provider has cancelled your booking."

	case "completed":
		notificationType = "booking_completed"
		notificationTitle = "Booking completed"
		notificationMessage = "Your appointment has been marked as completed."

	case "no_show":
		notificationType = "booking_no_show"
		notificationTitle = "Booking marked as no-show"
		notificationMessage = "The provider has marked this booking as a no-show."
	}

	_, err = r.notifications.Create(ctx, tx, notifications.CreateInput{
		UserID:    customerID,
		BookingID: &bookingID,
		Type:      notificationType,
		Title:     notificationTitle,
		Message:   notificationMessage,
	})

	if err != nil {
		return nil, fmt.Errorf("create provider status notification: %w", err)
	}

	// 6. Read the updated booking before committing.
	var booking ProviderBooking

	err = tx.QueryRow(ctx, `
		SELECT
			b.id,
			b.slot_id,
			b.customer_id,
			u.email,
			b.status,
			b.notes,
			s.starts_at,
			s.ends_at,
			b.created_at,
			b.updated_at
		FROM bookings b
		JOIN availability_slots s ON s.id = b.slot_id
		JOIN users u ON u.id = b.customer_id
		WHERE b.id = $1
	`, bookingID).Scan(
		&booking.ID,
		&booking.SlotID,
		&booking.CustomerID,
		&booking.CustomerEmail,
		&booking.Status,
		&booking.Notes,
		&booking.SlotStartsAt,
		&booking.SlotEndsAt,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("retrieve updated provider booking: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit booking status transaction: %w", err)
	}

	return &booking, nil
}

// listPublicProvidersWhere is the shared filter for the public provider
// directory. A provider is public when a profile row exists AND the
// account is active — deactivated providers vanish from the directory,
// the detail endpoint, and availability-filtered results alike. The
// search term arrives pre-wrapped as %term% with wildcards escaped
// (nil = no filter). The availability window uses overlap semantics: a
// provider matches when at least one open, unbooked slot intersects
// [from, to). All user input is bound as parameters; nothing is
// concatenated.
const publicProviderFilter = `
		WHERE ($1::varchar IS NULL OR pp.display_name ILIKE $1)
		  AND EXISTS (
		      SELECT 1
		      FROM users u
		      WHERE u.id = pp.user_id
		        AND u.is_active = TRUE
		  )
		  AND ($2::timestamptz IS NULL OR EXISTS (
		      SELECT 1
		      FROM availability_slots s
		      WHERE s.provider_id = pp.user_id
		        AND s.status = 'open'
		        AND s.starts_at < $3::timestamptz
		        AND s.ends_at > $2::timestamptz
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
		              AND tstzrange(up.starts_at, up.ends_at, '[)')
		                  && tstzrange(s.starts_at, s.ends_at, '[)')
		        )
		  ))`

func publicProviderSearchArg(search string) any {
	if search == "" {
		return nil
	}

	escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search)
	return "%" + escaped + "%"
}

// ListPublicProviders returns providers with a profile, newest profile
// first with a deterministic user_id tie-breaker.
func (r *Repository) ListPublicProviders(
	ctx context.Context,
	input ProviderListInput,
) ([]PublicProvider, error) {
	query := `
		SELECT
			pp.user_id,
			pp.display_name,
			pp.bio,
			pp.timezone,
			pp.created_at
		FROM provider_profiles pp
	` + publicProviderFilter + `
		ORDER BY pp.created_at DESC, pp.user_id ASC
		LIMIT $4
		OFFSET $5
	`

	rows, err := r.db.Query(
		ctx,
		query,
		publicProviderSearchArg(input.Search),
		input.AvailableFrom,
		input.AvailableTo,
		input.Limit,
		input.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list public providers: %w", err)
	}
	defer rows.Close()

	providers := make([]PublicProvider, 0)

	for rows.Next() {
		var provider PublicProvider

		err := rows.Scan(
			&provider.ID,
			&provider.DisplayName,
			&provider.Bio,
			&provider.Timezone,
			&provider.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan public provider: %w", err)
		}

		providers = append(providers, provider)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public providers: %w", err)
	}

	return providers, nil
}

// CountPublicProviders applies the same filters as ListPublicProviders
// so the total is independent of the requested page.
func (r *Repository) CountPublicProviders(
	ctx context.Context,
	input ProviderListInput,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM provider_profiles pp
	` + publicProviderFilter

	var total int64

	err := r.db.QueryRow(
		ctx,
		query,
		publicProviderSearchArg(input.Search),
		input.AvailableFrom,
		input.AvailableTo,
	).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("count public providers: %w", err)
	}

	return total, nil
}

// GetPublicProvider returns one provider's public profile. A missing row
// or a deactivated account yields ErrProfileNotFound regardless of
// whether the user account exists.
func (r *Repository) GetPublicProvider(
	ctx context.Context,
	providerID uuid.UUID,
) (*PublicProvider, error) {
	var provider PublicProvider

	err := r.db.QueryRow(ctx, `
		SELECT
			pp.user_id,
			pp.display_name,
			pp.bio,
			pp.timezone,
			pp.created_at
		FROM provider_profiles pp
		WHERE pp.user_id = $1
		  AND EXISTS (
		      SELECT 1
		      FROM users u
		      WHERE u.id = pp.user_id
		        AND u.is_active = TRUE
		  )
	`, providerID).Scan(
		&provider.ID,
		&provider.DisplayName,
		&provider.Bio,
		&provider.Timezone,
		&provider.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get public provider: %w", err)
	}

	return &provider, nil
}
