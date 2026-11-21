-- Entradas e saídas que se repetem todo mês fora o salário (aluguel, seguro, financiamento, a mãe pagando uma dívida
-- antiga, um valor em 10x, uma ajuda que cresce todo mês). O valor do mês é amount + step × (meses desde start_month),
-- limitado a max_amount; installments, se informado, é quantos meses dura. Uma transação com um dos nomes de payee_match
-- (";" separa), no mesmo sentido e com o valor do mês (±2%), conta como feita naquele mês.
CREATE TABLE IF NOT EXISTS recurring_flows (
    id           UUID          PRIMARY KEY,
    direction    TEXT          NOT NULL CHECK (direction IN ('IN', 'OUT')),
    name         TEXT          NOT NULL CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 60),
    party        TEXT          NOT NULL DEFAULT '' CHECK (CHAR_LENGTH(party) <= 60),
    amount       NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    step         NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (step >= 0),
    max_amount   NUMERIC(14,2) CHECK (max_amount > 0),
    day          INT           NOT NULL CHECK (day BETWEEN 1 AND 31),
    start_month  DATE          NOT NULL,
    installments INT           CHECK (installments BETWEEN 1 AND 120),
    payee_match  TEXT          NOT NULL DEFAULT '' CHECK (CHAR_LENGTH(payee_match) <= 200),
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
