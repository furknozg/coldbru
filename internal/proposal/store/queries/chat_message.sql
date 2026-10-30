-- name: InsertChatMessage :one
INSERT INTO chat_message (campaign_id, step_id, kind, body, options, reply_to)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetChatMessage :one
SELECT * FROM chat_message WHERE id = $1;

-- name: ListChatMessages :many
-- Oldest first. UUIDv7 ids sort by creation time (D-030).
SELECT * FROM chat_message WHERE campaign_id = $1 ORDER BY id;

-- name: ListChatMessagesOfStep :many
SELECT * FROM chat_message WHERE step_id = $1 ORDER BY id;

-- name: GetAnswerOfQuestion :one
SELECT * FROM chat_message WHERE reply_to = $1;
