BEGIN;

CREATE INDEX IF NOT EXISTS idx_chatgpt_messages_timestamp ON chatgpt_messages (timestamp);
CREATE INDEX IF NOT EXISTS idx_chatgpt_messages_user_name_timestamp ON chatgpt_messages (user_name, timestamp);
CREATE INDEX IF NOT EXISTS idx_chatgpt_threads_rel_sequence_num ON chatgpt_threads_rel (sequence_num);

COMMIT;