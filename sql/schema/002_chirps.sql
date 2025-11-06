-- +goose Up
CREATE TABLE chirps(
    id UUID PRIMARY KEY,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    body TEXT,
    user_id UUID
);


-- +goose Down
DROP TABLE chirps;