-- Ciclos do salário: cada ciclo vai do dia em que cai a 1ª parte do salário até a véspera da próxima. month é o mês que
-- dá nome ao ciclo (primeiro dia); starts_on, o dia em que ele começa. O app regrava a tabela inteira (no boot, quando o
-- salário configurado muda e depois de sincronizar). Vazia = mês do calendário.
CREATE TABLE IF NOT EXISTS cycles (
    month     DATE PRIMARY KEY,
    starts_on DATE NOT NULL UNIQUE
);

-- cycle_of é o ciclo (month) em que cai o dia d; antes do primeiro ciclo, ou sem ciclos, o mês do calendário.
CREATE OR REPLACE FUNCTION cycle_of(d DATE) RETURNS DATE LANGUAGE sql STABLE AS $$
    SELECT COALESCE(
        (SELECT c.month FROM cycles c WHERE c.starts_on <= d ORDER BY c.starts_on DESC LIMIT 1),
        DATE_TRUNC('month', d)::date)
$$;

-- cycle_next_start é o dia em que começa o ciclo seguinte ao ciclo key (o fim do ciclo é a véspera).
CREATE OR REPLACE FUNCTION cycle_next_start(key DATE) RETURNS DATE LANGUAGE sql STABLE AS $$
    SELECT COALESCE(
        (SELECT c.starts_on FROM cycles c WHERE c.month = (key + INTERVAL '1 month')::date),
        (key + INTERVAL '1 month')::date)
$$;
