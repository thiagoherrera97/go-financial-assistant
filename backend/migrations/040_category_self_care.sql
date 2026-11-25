-- Categoria Autocuidado (SELF_CARE): as regras de categoria (Classificar) passam a aceitá-la.
ALTER TABLE category_rules DROP CONSTRAINT IF EXISTS category_rules_category_check;
ALTER TABLE category_rules ADD CONSTRAINT category_rules_category_check CHECK (category IN
    ('FOOD', 'MARKET', 'TRANSPORT', 'HEALTH', 'ENTERTAINMENT', 'SHOPPING', 'HOUSING', 'BILLS', 'EDUCATION', 'PEOPLE',
     'SELF_CARE', 'OTHER'));
