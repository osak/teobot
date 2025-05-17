BEGIN;

ALTER TABLE chatgpt_messages
    DROP COLUMN IF EXISTS timestamp;

COMMIT;