-- Faturas fechadas dos cartões, como o banco informa pelo Open Finance (GET /bills do Pluggy): vencimento e total.
-- Servem para conferir a fatura: o que o banco cobrou e não chegou como lançamento vira um ajuste "não detalhado"
-- (pagamento com external_id 'finassist:bill:<id>'). Idempotente.
CREATE TABLE IF NOT EXISTS card_bills (
    external_id  TEXT          PRIMARY KEY,
    account_id   UUID          NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    due_date     DATE          NOT NULL,
    total_amount NUMERIC(12,2) NOT NULL,
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_card_bills_account_due ON card_bills (account_id, due_date);
