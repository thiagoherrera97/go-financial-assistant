-- Idempotente: a constraint de kind só é trocada se ainda não aceita TRANSFER; a de parcelamento só é criada se não existe.
DO $$
BEGIN
    -- Troca a constraint antiga, que só permite EXPENSE e INCOME, por uma que inclui TRANSFER
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'purchases'::regclass AND conname = 'purchases_kind_check'
          AND pg_get_constraintdef(oid) LIKE '%TRANSFER%'
    ) THEN
        ALTER TABLE purchases DROP CONSTRAINT IF EXISTS purchases_kind_check;
        ALTER TABLE purchases
            ADD CONSTRAINT purchases_kind_check
                CHECK (kind IN ('EXPENSE', 'INCOME', 'TRANSFER'));
    END IF;

    -- TRANSFER não pode ser parcelado
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'purchases'::regclass AND conname = 'chk_transfer_no_installment') THEN
        ALTER TABLE purchases
            ADD CONSTRAINT chk_transfer_no_installment
                CHECK (kind != 'TRANSFER' OR type != 'INSTALLMENT');
    END IF;
END $$;
