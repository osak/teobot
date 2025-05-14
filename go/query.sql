-- name: CreateChatgptMessage :exec
INSERT INTO chatgpt_messages (
    message_type, json_body, user_name, mastodon_status_id
) VALUES (?, ?, ?, ?);

-- name: CreateChatgptMessages :copyfrom
INSERT INTO chatgpt_messages (
    message_type, json_body, user_name, mastodon_status_id
) VALUES (?, ?, ?, ?);

-- name: GetLastInsertedChatgptMessage :one
SELECT *
FROM chatgpt_messages
WHERE id = LAST_INSERT_ID();

-- name: FindChatgptMessageByMastodonStatusId :one
SELECT *
FROM chatgpt_messages
WHERE mastodon_status_id = ?;

-- name: CreateChatgptThread :exec
INSERT INTO chatgpt_threads (id) VALUES (NULL);

-- name: GetLastInsertedChatgptThreadId :one
SELECT LAST_INSERT_ID() AS id;

-- name: CreateChatgptThreadRel :exec
INSERT INTO chatgpt_threads_rel (
    thread_id, chatgpt_message_id, sequence_num
) VALUES (?, ?, ?);

-- name: GetChatgptMessagesByThreadId :many
SELECT chatgpt_messages.*, chatgpt_threads_rel.sequence_num
FROM chatgpt_messages
INNER JOIN chatgpt_threads_rel ON chatgpt_messages.id = chatgpt_threads_rel.chatgpt_message_id
WHERE chatgpt_threads_rel.thread_id = ?;

-- name: GetChatgptThreadRels :many
SELECT *
FROM chatgpt_threads_rel
WHERE thread_id IN (SELECT DISTINCT thread_id FROM chatgpt_threads_rel WHERE chatgpt_threads_rel.chatgpt_message_id = ?)
ORDER BY thread_id, sequence_num;
