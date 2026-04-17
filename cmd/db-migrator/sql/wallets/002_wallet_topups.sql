CREATE TABLE IF NOT EXISTS wallet_topups (
    id            VARCHAR(36)    PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id       VARCHAR(36)    NOT NULL,
    amount        NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    balance_after NUMERIC(12, 2) NOT NULL CHECK (balance_after >= 0),
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallet_topups_user_created_at
    ON wallet_topups(user_id, created_at DESC, id DESC);
