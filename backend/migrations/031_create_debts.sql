-- Dívidas pessoais (ex.: com a família). Dois tipos:
--   OVERDRAFT    um buraco que a pessoa cobre com o cheque especial de uma conta (account_id) até o salário cair: o saldo
--                negativo dessa conta aparece como esta dívida, não como "negativo". due_day é o dia em que a conta costuma
--                entrar no limite (ex.: vencimento da fatura) e free_days, os dias sem juros que o banco dá.
--   INSTALLMENTS parcelas fixas (installment_amount, installments) a partir de first_due, todo due_day. Os Pix cuja
--                descrição contém payee_match abatem as parcelas sozinhos; paid_manual soma as marcadas à mão.
CREATE TABLE IF NOT EXISTS debts (
    id                 UUID          PRIMARY KEY,
    kind               TEXT          NOT NULL CHECK (kind IN ('OVERDRAFT', 'INSTALLMENTS')),
    name               TEXT          NOT NULL CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 60),
    creditor           TEXT          NOT NULL DEFAULT '' CHECK (CHAR_LENGTH(creditor) <= 60),
    note               TEXT          NOT NULL DEFAULT '' CHECK (CHAR_LENGTH(note) <= 120),
    original_amount    NUMERIC(14,2) NOT NULL CHECK (original_amount > 0),
    due_day            INT           NOT NULL CHECK (due_day BETWEEN 1 AND 31),
    account_id         UUID          REFERENCES accounts(id) ON DELETE SET NULL,
    free_days          INT           NOT NULL DEFAULT 0 CHECK (free_days BETWEEN 0 AND 60),
    installment_amount NUMERIC(14,2) CHECK (installment_amount > 0),
    installments       INT           CHECK (installments BETWEEN 1 AND 120),
    first_due          DATE,
    payee_match        TEXT          NOT NULL DEFAULT '' CHECK (CHAR_LENGTH(payee_match) <= 60),
    paid_manual        INT           NOT NULL DEFAULT 0 CHECK (paid_manual >= 0),
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CHECK (kind <> 'INSTALLMENTS' OR (installment_amount IS NOT NULL AND installments IS NOT NULL AND first_due IS NOT NULL))
);

-- Menor e último saldo de cada conta corrente por dia (fuso de São Paulo), gravados a cada sincronização. É o que diz
-- quantos dias a conta ficou no limite e quão fundo foi.
CREATE TABLE IF NOT EXISTS account_balance_days (
    account_id   UUID          NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    day          DATE          NOT NULL,
    min_balance  NUMERIC(14,2) NOT NULL,
    last_balance NUMERIC(14,2) NOT NULL,
    PRIMARY KEY (account_id, day)
);
