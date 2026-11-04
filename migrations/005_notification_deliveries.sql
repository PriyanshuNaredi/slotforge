
-- +goose Up

CREATE TABLE notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    notification_id UUID NOT NULL
        REFERENCES notifications(id) ON DELETE CASCADE,

    recipient_email VARCHAR(320) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'sent', 'failed')),

    attempts INTEGER NOT NULL DEFAULT 0
        CHECK (attempts >= 0),

    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    locked_at TIMESTAMPTZ,

    sent_at TIMESTAMPTZ,

    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT notification_deliveries_one_email_per_notification
        UNIQUE (notification_id)
);

-- Efficiently find pending deliveries ready to be processed.
CREATE INDEX notification_deliveries_pending_idx
    ON notification_deliveries (available_at, created_at)
    WHERE status = 'pending';

-- Find processing jobs whose worker lock has expired.
CREATE INDEX notification_deliveries_processing_idx
    ON notification_deliveries (locked_at)
    WHERE status = 'processing';

-- +goose Down

DROP TABLE IF EXISTS notification_deliveries;
