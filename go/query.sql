-- name: CreateChatgptMessage :one
INSERT INTO chatgpt_messages (
    id, message_type, json_body, user_name, mastodon_status_id, timestamp, privacy_level
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: FindChatgptMessageByMastodonStatusId :one
SELECT *
FROM chatgpt_messages
WHERE mastodon_status_id = $1;

-- name: CreateChatgptThread :one
INSERT INTO chatgpt_threads (id) VALUES ($1)
RETURNING *;

-- name: GetChatgptMessagesByThreadId :many
SELECT
    json_body,
    user_name
FROM chatgpt_messages
INNER JOIN chatgpt_threads_rel ON chatgpt_messages.id = chatgpt_threads_rel.chatgpt_message_id
WHERE chatgpt_threads_rel.thread_id = $1
AND message_type != 'pseudo_message'
ORDER BY chatgpt_threads_rel.sequence_num;

-- name: CreateChatgptThreadRel :exec
INSERT INTO chatgpt_threads_rel (
    thread_id, chatgpt_message_id, sequence_num
) VALUES ($1, $2, $3);

-- name: GetChatgptThreadRels :many
SELECT *
FROM chatgpt_threads_rel
WHERE thread_id IN (SELECT DISTINCT thread_id FROM chatgpt_threads_rel WHERE chatgpt_threads_rel.chatgpt_message_id = $1)
ORDER BY thread_id, sequence_num;

-- name: GetMaxSequenceNum :one
SELECT COALESCE(MAX(sequence_num), 0)::INT AS max_sequence_num
FROM chatgpt_threads_rel
WHERE thread_id = $1;

-- name: GetRecentThreadIdsByUserName :many
SELECT DISTINCT thread_id
FROM chatgpt_threads_rel
WHERE chatgpt_message_id IN (
    SELECT id
    FROM chatgpt_messages
    WHERE user_name = $1
    AND message_type != 'pseudo_message'
    ORDER BY timestamp DESC
    LIMIT $2
);