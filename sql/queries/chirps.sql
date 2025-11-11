-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;
-- name: GetSingleChirp :one
SELECT * FROM chirps
WHERE id = $1;

-- name: GetChirpsAscByCreated :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: DeleteChirps :exec
DELETE FROM chirps;

-- name: DeleteSingleChirp :exec
DELETE FROM chirps
WHERE id = $1;