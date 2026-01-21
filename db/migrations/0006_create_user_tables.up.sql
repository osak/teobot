BEGIN;

CREATE TABLE users(
    id UUID NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE chatgpt_messages ADD COLUMN user_id UUID REFERENCES users(id);

CREATE TABLE mastodon_user_mappings(
    mastodon_account_id TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_mastodon_user_mappings_updated_at
    BEFORE UPDATE ON mastodon_user_mappings
    FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

COMMIT;