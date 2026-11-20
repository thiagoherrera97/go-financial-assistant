-- Nota escrita pela pessoa em cada transação ("o que foi"). A descrição do banco continua intacta.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS note TEXT CHECK (note IS NULL OR char_length(note) <= 300);

-- Nota mandada pelo chat antes de a transação chegar do banco: aplicada quando a sincronização a trouxer.
CREATE TABLE IF NOT EXISTS pending_notes (
    id         UUID          PRIMARY KEY,
    amount     NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    day        DATE          NOT NULL,
    note       TEXT          NOT NULL CHECK (char_length(note) BETWEEN 1 AND 300),
    created_at TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
