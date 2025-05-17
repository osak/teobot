BEGIN;

ALTER TABLE chatgpt_messages
    DROP COLUMN IF EXISTS privacy_level;

COMMIT;