-- Revisão de transferências (Pix/TED que o Pluggy classificou como "Transfer ..."): a decisão da pessoa sobre cada
-- pagamento e o que ele era antes, para desfazer. Idempotente (IF NOT EXISTS), como todas as migrations.
--   OWN     transferência entre contas suas: o pagamento vira CANCELLED e deixa de contar;
--   PERSON  para uma pessoa: a categoria vira PEOPLE (o tipo, despesa ou renda, não muda);
--   PAYMENT pagamento: a categoria escolhida (o tipo não muda).
-- applied_category é a categoria que a decisão aplicou (PERSON e PAYMENT; nula em OWN): desfazer só restaura a anterior
-- se a categoria atual ainda é essa (trocada depois em outra tela, fica como está).
-- apply_to_future fica reservada para "valer para os próximos desta contraparte" (ainda sempre falsa).
CREATE TABLE IF NOT EXISTS transfer_decisions (
    payment_id        UUID        PRIMARY KEY REFERENCES payments(id) ON DELETE CASCADE,
    decision          TEXT        NOT NULL CHECK (decision IN ('OWN', 'PERSON', 'PAYMENT')),
    previous_status   TEXT        NOT NULL CHECK (previous_status IN ('PENDING', 'PAID', 'CANCELLED')),
    previous_category TEXT        NOT NULL,
    applied_category  TEXT,
    apply_to_future   BOOLEAN     NOT NULL DEFAULT FALSE,
    decided_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Banco de desenvolvimento que já tinha a tabela sem a coluna.
ALTER TABLE transfer_decisions ADD COLUMN IF NOT EXISTS applied_category TEXT;

CREATE INDEX IF NOT EXISTS idx_transfer_decisions_decided_at ON transfer_decisions (decided_at DESC);
