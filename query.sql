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

-- name: GetRecentChatgptMessages :many
SELECT
    json_body,
    user_name
FROM chatgpt_messages
WHERE message_type != 'pseudo_message'
AND privacy_level != 'private'
ORDER BY timestamp DESC
LIMIT $1;

-- name: GetFullChatgptMessagesByThreadId :many
SELECT chatgpt_messages.*
FROM chatgpt_messages
 INNER JOIN chatgpt_threads_rel ON chatgpt_messages.id = chatgpt_threads_rel.chatgpt_message_id
WHERE chatgpt_threads_rel.thread_id = $1
  AND message_type != 'pseudo_message'
ORDER BY chatgpt_threads_rel.sequence_num;

-- name: GetRecentFullChatgptMessages :many
SELECT *
FROM chatgpt_messages
WHERE message_type != 'pseudo_message'
AND privacy_level != 'private'
ORDER BY timestamp DESC
LIMIT $1;

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

-- name: CreateUser :exec
INSERT INTO users (id, name)
VALUES($1, $2);

-- name: CreateMastodonUserMapping :exec
INSERT INTO mastodon_user_mappings (mastodon_account_id, user_id)
VALUES($1, $2);

-- name: GetUserByMastodonAccountId :one
SELECT users.*
FROM users
INNER JOIN mastodon_user_mappings ON users.id = mastodon_user_mappings.user_id
WHERE mastodon_user_mappings.mastodon_account_id = $1;

-- name: UpdateMastodonStatusId :exec
UPDATE chatgpt_messages SET mastodon_status_id = $1
WHERE id = $2;

-- name: CreateImage :exec
INSERT INTO images (id, url)
VALUES($1, $2);

-- name: CreateChatGptMessageImageRel :exec
INSERT INTO chatgpt_message_image_rel (chatgpt_message_id, image_id)
VALUES($1, $2);

-- name: GetImagesByChatGptMessageId :many
SELECT images.*
FROM images
INNER JOIN chatgpt_message_image_rel AS cmir ON images.id = cmir.image_id
WHERE cmir.chatgpt_message_id = $1;

-- name: GetImagesByChatGptMessageIds :many
SELECT
    rel.chatgpt_message_id,
    rel.image_id,
    images.*
FROM chatgpt_message_image_rel AS rel
INNER JOIN images ON rel.image_id = images.id
WHERE rel.chatgpt_message_id = ANY($1 :: UUID[]);
