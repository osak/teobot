CREATE TABLE chatgpt_message (
    id SERIAL PRIMARY KEY,
    message_type VARCHAR(32) NOT NULL,
    json_body TEXT NOT NULL,
    user_name VARCHAR(64) NOT NULL,

    thread_root_message_id INT,
    parent_message_id INT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE mastodon_message (
    id SERIAL PRIMARY KEY,
    status_id VARCHAR(32) NOT NULL,
    json_body TEXT NOT NULL,
    user_name VARCHAR(64) NOT NULL,

    chatgpt_message_id INT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);