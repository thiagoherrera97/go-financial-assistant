-- Idempotente: pode rodar de novo em um banco que já tem estas mudanças (inclusive depois da 003).
ALTER TABLE purchases
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'EXPENSE'
        CHECK (kind IN ('EXPENSE', 'INCOME'));

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'purchases'::regclass AND conname = 'chk_income_no_installment') THEN
        ALTER TABLE purchases
            ADD CONSTRAINT chk_income_no_installment
                CHECK (kind != 'INCOME' OR type != 'INSTALLMENT');
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_purchases_kind ON purchases(kind);
