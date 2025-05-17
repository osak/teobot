BEGIN;

DROP TRIGGER IF EXISTS update_chatgpt_messages_updated_at ON chatgpt_messages;
DROP TRIGGER IF EXISTS update_chatgpt_threads_updated_at ON chatgpt_threads;
DROP TRIGGER IF EXISTS update_chatgpt_threads_rel_updated_at ON chatgpt_threads_rel;
DROP FUNCTION IF EXISTS set_updated_at();

COMMIT;