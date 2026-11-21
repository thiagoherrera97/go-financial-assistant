-- Fornecedores de um evento planejado (ex.: casamento): valor contratado, pagamentos feitos e até quando quitar.
-- Os totais (falta pagar, quanto guardar por mês) não são gravados: são calculados na hora.

-- Plano único (uma linha): nome do evento, dia do aporte mensal e onde está o dinheiro guardado.
-- savings_box é o nome de uma caixinha da conta corrente; quando nulo, vale saved_manual.
CREATE TABLE IF NOT EXISTS vendor_plan (
    id               BOOLEAN       PRIMARY KEY DEFAULT TRUE CHECK (id),
    name             TEXT          NOT NULL DEFAULT 'Casamento' CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 60),
    contribution_day INT           NOT NULL DEFAULT 10 CHECK (contribution_day BETWEEN 1 AND 28),
    savings_box      TEXT          CHECK (savings_box IS NULL OR CHAR_LENGTH(savings_box) BETWEEN 1 AND 120),
    saved_manual     NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (saved_manual >= 0)
);
INSERT INTO vendor_plan (id) VALUES (TRUE) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS vendors (
    id         UUID          PRIMARY KEY,
    name       TEXT          NOT NULL CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 60),
    contracted NUMERIC(14,2) NOT NULL CHECK (contracted > 0),
    due_date   DATE,         -- último dia para quitar; nulo = sem prazo
    created_at TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Pagamento a um fornecedor. Lançado à mão (payment_id nulo) ou ligado a uma transação do extrato: aí valor e data
-- são copiados dela, e o vínculo some se a transação for apagada. Uma transação paga um fornecedor só.
CREATE TABLE IF NOT EXISTS vendor_payments (
    id         UUID          PRIMARY KEY,
    vendor_id  UUID          NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    amount     NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    paid_on    DATE          NOT NULL,
    payment_id UUID          UNIQUE REFERENCES payments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vendor_payments_vendor ON vendor_payments(vendor_id);
