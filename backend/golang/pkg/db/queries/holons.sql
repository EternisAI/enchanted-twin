-- Thread queries
-- name: GetThreads :many
SELECT t.id, t.title, t.content, t.author_identity, t.created_at, t.expires_at, 
       t.image_urls, t.actions, t.views,
       a.identity, a.alias
FROM threads t
JOIN authors a ON t.author_identity = a.identity
ORDER BY t.created_at DESC
LIMIT ? OFFSET ?;

-- name: GetThread :one
SELECT t.id, t.title, t.content, t.author_identity, t.created_at, t.expires_at, 
       t.image_urls, t.actions, t.views,
       a.identity, a.alias
FROM threads t
JOIN authors a ON t.author_identity = a.identity
WHERE t.id = ?;

-- name: CreateThread :exec
INSERT INTO threads (id, title, content, author_identity, created_at, expires_at, image_urls, actions, views)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0);

-- name: IncrementThreadViews :exec
UPDATE threads SET views = views + 1 WHERE id = ?;

-- Thread message queries
-- name: GetThreadMessages :many
SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
       tm.is_delivered, tm.actions,
       a.identity, a.alias
FROM thread_messages tm
JOIN authors a ON tm.author_identity = a.identity
WHERE tm.thread_id = ?
ORDER BY tm.created_at ASC;

-- name: CreateThreadMessage :exec
INSERT INTO thread_messages (id, thread_id, author_identity, content, created_at, is_delivered, actions)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetThreadMessage :one
SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
       tm.is_delivered, tm.actions,
       a.identity, a.alias
FROM thread_messages tm
JOIN authors a ON tm.author_identity = a.identity
WHERE tm.id = ?;

-- Holon queries
-- name: GetUserHolons :many
SELECT h.name 
FROM holons h
INNER JOIN holon_participants hp ON h.id = hp.holon_id
WHERE hp.author_identity = ?
ORDER BY h.name;

-- name: GetHolonByIdentifier :one
SELECT id FROM holons 
WHERE id = ? OR name = ? 
LIMIT 1;

-- name: CheckUserInHolon :one
SELECT EXISTS(SELECT 1 FROM holon_participants WHERE holon_id = ? AND author_identity = ?);

-- name: AddUserToHolon :exec
INSERT INTO holon_participants (holon_id, author_identity) 
VALUES (?, ?)
ON CONFLICT (holon_id, author_identity) DO NOTHING;

-- Author queries
-- name: CreateOrUpdateAuthor :exec
INSERT INTO authors (identity, alias) 
VALUES (?, ?)
ON CONFLICT (identity) DO UPDATE SET alias = EXCLUDED.alias;

-- name: EnsureAuthorExists :exec
INSERT INTO authors (identity, alias) 
VALUES (?, ?)
ON CONFLICT (identity) DO NOTHING;

-- name: GetAuthor :one
SELECT identity, alias FROM authors WHERE identity = ?;
