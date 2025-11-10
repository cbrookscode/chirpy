-- +goose up
CREATE TABLE refresh_tokens (
    token TEXT PRIMARY KEY NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP,
    user_id UUID NOT NULL REFERENCES users(id),
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP
);

-- +goose down
DROP TABLE refresh_tokens;