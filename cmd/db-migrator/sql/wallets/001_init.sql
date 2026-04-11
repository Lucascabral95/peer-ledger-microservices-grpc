CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS wallets (
    id         VARCHAR(36)    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    VARCHAR(36)    NOT NULL UNIQUE,
    balance    NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key        VARCHAR(100) PRIMARY KEY,
    result     TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO wallets (user_id, balance) VALUES
    ('user-001', 100000.00),
    ('user-002',  50000.00),
    ('user-003',  75000.00),
    ('user-004',  29000.00)
ON CONFLICT (user_id) DO NOTHING;
