BEGIN;

CREATE TABLE images (
    id UUID NOT NULL PRIMARY KEY,
    url TEXT NOT NULL,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_images_updated_at
    BEFORE UPDATE ON images
    FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE chatgpt_message_image_rel (
    chatgpt_message_id UUID NOT NULL REFERENCES chatgpt_messages(id),
    image_id UUID NOT NULL REFERENCES images(id),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_chatgpt_message_image_rel_updated_at
    BEFORE UPDATE ON chatgpt_message_image_rel
    FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

COMMIT;
