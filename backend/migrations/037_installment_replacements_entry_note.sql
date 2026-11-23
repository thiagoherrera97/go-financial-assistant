-- Nota que acompanha o gasto que substitui a série trocada (ex.: "Compras Semar").
ALTER TABLE installment_replacements ADD COLUMN IF NOT EXISTS entry_note TEXT CHECK (char_length(entry_note) <= 300);
