-- Subcategoria da despesa (ex.: Alimentação › Delivery) e se ela é essencial ou supérfluo, para o planejamento mostrar
-- o supérfluo que mais pesa. O catálogo fica no código (domain/subcategory.go); o app preenche as duas colunas sozinho a
-- cada minuto (pela descrição e pela categoria do banco) e só não mexe no que você corrigiu à mão (*_manual).
-- essential NULL = depende do caso (o app pede para você revisar).
ALTER TABLE purchases ADD COLUMN IF NOT EXISTS subcategory        TEXT;
ALTER TABLE purchases ADD COLUMN IF NOT EXISTS essential          BOOLEAN;
ALTER TABLE purchases ADD COLUMN IF NOT EXISTS subcategory_manual BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE purchases ADD COLUMN IF NOT EXISTS essential_manual   BOOLEAN NOT NULL DEFAULT FALSE;

-- A regra por estabelecimento também pode fixar a subcategoria e o essencial/supérfluo ("aplicar a todas desse
-- estabelecimento"). NULL = o app decide.
ALTER TABLE category_rules ADD COLUMN IF NOT EXISTS subcategory TEXT;
ALTER TABLE category_rules ADD COLUMN IF NOT EXISTS essential   BOOLEAN;
