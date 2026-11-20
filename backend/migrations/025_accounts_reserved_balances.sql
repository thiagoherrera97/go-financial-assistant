-- Caixinhas da conta corrente (ex.: Mercado Pago): saldo separado que rende e volta para a conta quando a pessoa quer.
-- Lista de {name, amount}, como o banco informa (bankData.reservedBalances do Pluggy).
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS reserved_balances JSONB NOT NULL DEFAULT '[]';
