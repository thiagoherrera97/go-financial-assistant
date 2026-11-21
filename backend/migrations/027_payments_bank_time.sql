-- Momento exato em que o banco registrou a transação (Open Finance), com hora. Só para mostrar ("de onde veio"): as
-- contas do app continuam usando o dia (due_date, paid_at). NULL em manuais, parcelas e quando o banco não manda a hora.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS bank_time TIMESTAMPTZ;
