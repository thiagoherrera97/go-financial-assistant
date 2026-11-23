-- Último mês (primeiro dia) em que o gasto da troca já foi lançado: a sincronização só olha os meses depois dele e, depois
-- da última parcela da série, nem lê mais a regra.
ALTER TABLE installment_replacements ADD COLUMN IF NOT EXISTS last_entry_month DATE;
