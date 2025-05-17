BEGIN;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_chatgpt_messages_updated_at
BEFORE UPDATE ON chatgpt_messages
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_chatgpt_threads_updated_at
BEFORE UPDATE ON chatgpt_threads
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_chatgpt_threads_rel_updated_at
BEFORE UPDATE ON chatgpt_threads_rel
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

COMMIT;