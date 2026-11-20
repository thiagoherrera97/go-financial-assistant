-- Mês da fatura do cartão. bill_due_date é o vencimento da fatura em que a compra no cartão cai; quando preenchida, é ela
-- que decide o mês do pagamento (como na planilha: a compra entra no mês em que a fatura é paga). due_date continua sendo
-- o dia da compra, que é o mostrado nas listas. Nula para tudo que não é cartão ou não tem conta com vencimento.
-- O app preenche e recalcula a coluna a cada sincronização (openfinance: RecomputeCardBills). Idempotente.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS bill_due_date DATE;
