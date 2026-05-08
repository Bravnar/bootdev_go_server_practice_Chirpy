-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
  gen_random_uuid(),
  NOW(),
  NOW(),
  $1,
  $2
  )
  RETURNING *;

-- name: DeleteUsers :exec
DELETE FROM users;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE $1 = users.email;

-- name: GetUserByRefreshToken :one
SELECT users.*
FROM users
INNER JOIN refresh_tokens ON refresh_tokens.user_id = users.id
WHERE refresh_tokens.token = $1 AND
refresh_tokens.revoked_at IS NULL AND
refresh_tokens.expires_at > NOW();

-- name: UpdateUserEmailAndPass :one
UPDATE users
SET email = $1,
hashed_password = $2,
updated_at = NOW()
WHERE id = $3
RETURNING *;

-- name: UpdateUserStatus :one
UPDATE users
SET is_chirpy_red = true,
updated_at = NOW()
WHERE id = $1
RETURNING *;

