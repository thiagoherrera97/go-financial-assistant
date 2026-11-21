-- payee_match passa a aceitar vários nomes separados por ";" (o mesmo credor aparece com nomes diferentes no Pix e na TED).
ALTER TABLE debts DROP CONSTRAINT IF EXISTS debts_payee_match_check;
ALTER TABLE debts ADD CONSTRAINT debts_payee_match_check CHECK (CHAR_LENGTH(payee_match) <= 200);
