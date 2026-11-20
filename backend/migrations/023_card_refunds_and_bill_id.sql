-- Fatura informada pelo banco para cada transação do cartão (billId do Pluggy) e estornos do cartão como despesa negativa.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_bill_id TEXT;

-- Estorno no cartão é gravado com valor negativo (abate a fatura e a categoria). Zero continua proibido.
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_amount_check;
ALTER TABLE payments ADD CONSTRAINT payments_amount_check CHECK (amount <> 0);
ALTER TABLE purchases DROP CONSTRAINT IF EXISTS purchases_total_amount_check;
ALTER TABLE purchases ADD CONSTRAINT purchases_total_amount_check CHECK (total_amount <> 0);
