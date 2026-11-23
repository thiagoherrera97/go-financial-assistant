-- Parcelas de cartão trocadas por um lançamento manual (ex.: uma fatura parcelada que a pessoa prefere ver como um gasto
-- fixo por mês). Cada linha pega a série "(n/installments)" do cartão account_id cuja 1ª parcela caiu na fatura de
-- first_bill e a oculta (CANCELLED) a partir da fatura de from_bill: as reais que chegam do banco, as previstas pelo app e
-- sem gerar "ajuste da fatura" por elas. As faturas antes de from_bill ficam como estão.
CREATE TABLE IF NOT EXISTS installment_replacements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    installments INTEGER NOT NULL CHECK (installments BETWEEN 2 AND 120),
    first_bill   DATE NOT NULL,
    from_bill    DATE NOT NULL CHECK (from_bill >= first_bill),
    note         TEXT NOT NULL DEFAULT '' CHECK (char_length(note) <= 120),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, installments, first_bill)
);
