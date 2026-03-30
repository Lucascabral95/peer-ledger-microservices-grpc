CREATE DATABASE transactions_db;

\c transactions_db;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE transactions (
    id               VARCHAR(36)    PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id        VARCHAR(36)    NOT NULL,
    receiver_id      VARCHAR(36)    NOT NULL,
    amount           NUMERIC(12, 2) NOT NULL,
    idempotency_key  VARCHAR(100)   NOT NULL UNIQUE,
    status           VARCHAR(20)    NOT NULL DEFAULT 'completed',
    created_at       TIMESTAMP      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_sender   ON transactions(sender_id);
CREATE INDEX idx_transactions_receiver ON transactions(receiver_id);
