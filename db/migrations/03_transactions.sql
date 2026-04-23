CREATE DATABASE transactions_db;

\c transactions_db;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE transactions (
    id               VARCHAR(36)    PRIMARY KEY,
    sender_id        VARCHAR(36)    NOT NULL,
    receiver_id      VARCHAR(36)    NOT NULL,
    amount           NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    idempotency_key  VARCHAR(100)   NOT NULL UNIQUE,
    status           VARCHAR(20)    NOT NULL DEFAULT 'completed',
    sender_balance_after   NUMERIC(12, 2),
    receiver_balance_after NUMERIC(12, 2),
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_sender_created_at
    ON transactions(sender_id, created_at DESC);

CREATE INDEX idx_transactions_receiver_created_at
    ON transactions(receiver_id, created_at DESC);
