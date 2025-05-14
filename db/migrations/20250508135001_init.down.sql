DROP INDEX idx_chatgpt_threads_rel_message_id ON chatgpt_threads_rel;
DROP INDEX idx_chatgpt_threads_rel_thread_id ON chatgpt_threads_rel;
DROP TABLE IF EXISTS chatgpt_threads_rel;

DROP TABLE IF EXISTS chatgpt_threads;

DROP INDEX idx_chatgpt_messages_mastodon_status_id ON chatgpt_messages;
DROP TABLE IF EXISTS chatgpt_messages;