CREATE TABLE IF NOT EXISTS tasks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    goal TEXT NOT NULL,
    chain_action VARCHAR(255) DEFAULT '',
    address VARCHAR(255) DEFAULT '',
    thought TEXT NOT NULL,
    reply TEXT NOT NULL,
    chain_id VARCHAR(66) DEFAULT '',
    block_number VARCHAR(66) DEFAULT '',
    observes TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_tasks_created_at (created_at)
);
