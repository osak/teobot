BEGIN;

ALTER TABLE chatgpt_messages
    ADD COLUMN IF NOT EXISTS privacy_level VARCHAR(255) DEFAULT 'private';

COMMIT;