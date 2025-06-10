-- Config queries
-- name: GetConfigValue :one
SELECT value FROM config WHERE key = ?;

-- name: SetConfigValue :exec
INSERT OR REPLACE INTO config (key, value) VALUES (?, ?);

-- name: GetAllConfigKeys :many
SELECT key FROM config;
