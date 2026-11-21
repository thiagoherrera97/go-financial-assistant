-- No cartão, o Pluggy manda o horário de Brasília marcado como UTC ("00:26:33Z" é 00:26 no app do Santander). bank_time
-- foi gravado como UTC de verdade, 3h antes, e a 028 levou para o dia anterior as compras do cartão feitas antes das 3h.
-- Aqui o relógio gravado passa a ser lido como horário de São Paulo e o dia volta a ser o do relógio do cartão.

-- Primeiro o dia (usa o bank_time antigo): só lançamentos criados pela sincronização, cujo dia é o calculado pelo app.
UPDATE payments pay SET
    due_date = (pay.bank_time AT TIME ZONE 'UTC')::date,
    paid_at  = CASE WHEN pay.paid_at IS NULL THEN NULL
                    ELSE ((pay.bank_time AT TIME ZONE 'UTC')::date)::timestamp AT TIME ZONE 'UTC' END
FROM purchases p, accounts a
WHERE p.id = pay.purchase_id
  AND a.id = pay.account_id
  AND a.type = 'CREDIT'
  AND p.raw_input LIKE '[open finance:%'
  AND pay.bank_time IS NOT NULL
  AND pay.installment_number IS NULL
  AND pay.due_date = (pay.bank_time AT TIME ZONE 'America/Sao_Paulo')::date
  AND (pay.bank_time AT TIME ZONE 'UTC')::date <> pay.due_date;

-- Depois a hora: o relógio gravado (em UTC) é o de São Paulo.
UPDATE payments pay SET bank_time = (pay.bank_time AT TIME ZONE 'UTC') AT TIME ZONE 'America/Sao_Paulo'
FROM accounts a
WHERE a.id = pay.account_id AND a.type = 'CREDIT' AND pay.bank_time IS NOT NULL;
