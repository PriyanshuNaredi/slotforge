
-- +goose Up

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    booking_id UUID
        REFERENCES bookings(id) ON DELETE SET NULL,

    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,

    -- In-app read state is independent of delivery state.
    read_at TIMESTAMPTZ,

    -- Email delivery lifecycle.
    delivery_status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (
            delivery_status IN (
                'pending',
                'processing',
                'sent',
                'failed'
            )
        ),

    attempts INTEGER NOT NULL DEFAULT 0
        CHECK (attempts >= 0),

    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Efficiently find notifications waiting for delivery.
CREATE INDEX notifications_pending_delivery_idx
    ON notifications (available_at, created_at)
    WHERE delivery_status = 'pending';

-- Efficiently retrieve a user's notification history.
CREATE INDEX notifications_user_created_idx
    ON notifications (user_id, created_at DESC, id DESC);

-- Efficiently retrieve notifications associated with a booking.
CREATE INDEX notifications_booking_idx
    ON notifications (booking_id)
    WHERE booking_id IS NOT NULL;


-- +goose Down

DROP TABLE IF EXISTS notifications;
