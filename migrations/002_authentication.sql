-- +goose Up

-- Store only hashes of refresh tokens, never the raw tokens.
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    token_hash BYTEA NOT NULL UNIQUE,

    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX refresh_tokens_user_id_idx
    ON refresh_tokens (user_id);

CREATE INDEX refresh_tokens_expiry_idx
    ON refresh_tokens (expires_at);

-- +goose Down

DROP TABLE IF EXISTS refresh_tokens;
