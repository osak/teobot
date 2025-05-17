BEGIN;

DROP INDEX IF EXISTS idx_chatgpt_messages_timestamp;
DROP INDEX IF EXISTS idx_chatgpt_messages_user_name_timestamp;
DROP INDEX IF EXISTS idx_chatgpt_threads_rel_sequence_num;

COMMIT;