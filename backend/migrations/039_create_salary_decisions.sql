-- O que a pessoa disse que um crédito de salário é (domain.SalaryKind): o salário do mês inteiro (month), uma parte
-- (month e day), ou um extra que nunca conta como parte (hora extra, PLR, 13º, férias, outro). Sem linha, o app decide pelo
-- valor e pela data e, quando só adivinha, pede para conferir em Classificar.
CREATE TABLE IF NOT EXISTS salary_decisions (
    payment_id UUID        PRIMARY KEY REFERENCES payments(id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL CHECK (kind IN ('MONTH', 'PART', 'OVERTIME', 'PLR', 'THIRTEENTH', 'VACATION', 'OTHER')),
    month      DATE        CHECK (month IS NULL OR EXTRACT(DAY FROM month) = 1),
    day        INT         CHECK (day BETWEEN 1 AND 31),
    decided_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (kind NOT IN ('MONTH', 'PART') OR month IS NOT NULL),
    CHECK (kind <> 'PART' OR day IS NOT NULL)
);
