ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS sender_balance_after NUMERIC(12, 2),
    ADD COLUMN IF NOT EXISTS receiver_balance_after NUMERIC(12, 2);
