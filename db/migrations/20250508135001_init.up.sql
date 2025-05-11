CREATE TABLE chatgpt_messages (
    id SERIAL PRIMARY KEY,
    message_type VARCHAR(32) NOT NULL,
    json_body TEXT NOT NULL,
    user_name VARCHAR(64) NOT NULL,

    mastodon_status_id VARCHAR(32),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
CREATE INDEX idx_chatgpt_messages_mastodon_status_id ON chatgpt_messages(mastodon_status_id);

CREATE TABLE chatgpt_threads (
    id SERIAL PRIMARY KEY,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE chatgpt_threads_rel (
    thread_id INT NOT NULL,
    chatgpt_message_id INT NOT NULL,
    sequence_num INT NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
CREATE INDEX idx_chatgpt_threads_rel_thread_id ON chatgpt_threads_rel(thread_id);
CREATE INDEX idx_chatgpt_threads_rel_message_id ON chatgpt_threads_rel(chatgpt_message_id);