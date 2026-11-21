-- O dia das transações do banco passava a ser o de UTC: compra depois das 21h em Brasília caía no dia seguinte. A
-- sincronização agora usa o dia de São Paulo; aqui corrigimos o que já foi gravado e tem a hora do banco (bank_time),
-- só nos lançamentos criados pela sincronização e só quando o dia gravado é o de UTC (nunca mexe em data digitada).
UPDATE payments pay SET
    due_date = (pay.bank_time AT TIME ZONE 'America/Sao_Paulo')::date,
    paid_at  = CASE WHEN pay.paid_at IS NULL THEN NULL
                    ELSE ((pay.bank_time AT TIME ZONE 'America/Sao_Paulo')::date)::timestamp AT TIME ZONE 'UTC' END
FROM purchases p
WHERE p.id = pay.purchase_id
  AND p.raw_input LIKE '[open finance:%'
  AND pay.bank_time IS NOT NULL
  AND pay.installment_number IS NULL
  AND pay.due_date = (pay.bank_time AT TIME ZONE 'UTC')::date
  AND (pay.bank_time AT TIME ZONE 'America/Sao_Paulo')::date <> pay.due_date;
