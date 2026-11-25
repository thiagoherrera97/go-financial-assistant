-- Decisão automática de transferências pelo nome (domain.AutoTransferDecision): a decisão guarda se foi automática, e
-- desfazer uma transferência a tira das automáticas para sempre (a pessoa quer decidir aquela à mão).
ALTER TABLE transfer_decisions ADD COLUMN IF NOT EXISTS auto BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS transfer_auto_skips (
    payment_id UUID        PRIMARY KEY REFERENCES payments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
