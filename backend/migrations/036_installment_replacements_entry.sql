-- O lançamento que entra no lugar da série trocada (opcional): description, category e amount no dia day do mês de cada
-- fatura. O app só cria o da fatura já aberta, um por vez (não projeta os meses seguintes).
ALTER TABLE installment_replacements ADD COLUMN IF NOT EXISTS description TEXT CHECK (char_length(description) BETWEEN 1 AND 60);
ALTER TABLE installment_replacements ADD COLUMN IF NOT EXISTS category TEXT;
ALTER TABLE installment_replacements ADD COLUMN IF NOT EXISTS amount NUMERIC(12,2) CHECK (amount > 0);
ALTER TABLE installment_replacements ADD COLUMN IF NOT EXISTS day INTEGER CHECK (day BETWEEN 1 AND 31);
