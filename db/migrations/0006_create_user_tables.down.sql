BEGIN;

ALTER TABLE chatgpt_messages DROP COLUMN IF EXISTS user_id;
DROP TABLE IF EXISTS mastodon_user_mappings;
DROP TABLE IF EXISTS users;

COMMIT;