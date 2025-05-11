DROP INDEX IF EXISTS idx_chatgpt_threads_rel_message_id;
DROP INDEX IF EXISTS idx_chatgpt_threads_rel_thread_id;
DROP TABLE IF EXISTS chatgpt_threads_rel;

DROP TABLE IF EXISTS chatgpt_threads;

DROP INDEX IF EXISTS idx_chatgpt_messages_mastodon_status_id;
DROP TABLE IF EXISTS chatgpt_messages;