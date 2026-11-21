package domain

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		category    Category
		pluggy      string
		description string
		want        Subcategory
	}{
		{CategoryFood, "Food delivery", "IFD*BR", "FOOD_DELIVERY"},
		{CategoryFood, "Eating out", "99Food *Bullguer - Alto d", "FOOD_DELIVERY"},
		{CategoryFood, "Eating out", "KEETABR*VENETO RESTAURANT", "FOOD_DELIVERY"},
		{CategoryFood, "Eating out", "MARMITEX DO GORDINHO", "FOOD_DAILY"},
		{CategoryFood, "Eating out", "BLENZ CAFE", "FOOD_SNACK"},
		{CategoryFood, "Eating out", "McDonalds - Arcos Dourado", "FOOD_SNACK"},
		{CategoryFood, "Eating out", "TIO BASILIO RESTAURANT", "FOOD_RESTAURANT"},
		{CategoryFood, "Eating out", "BAR E CHOPERIA LAS BIR", "FOOD_RESTAURANT"},
		{CategoryFood, "Eating out", "GOLDEN SAO JOSE GRILL", "FOOD_RESTAURANT"},
		{CategoryFood, "Eating out", "DEX", "FOOD_RESTAURANT"}, // sem pista na descrição, vale o Pluggy
		{CategoryFood, "", "Pix enviado Fulano", "FOOD_OTHER"},
		{CategoryMarket, "Groceries", "CARREFOUR EXPRESS SANT", "MARKET_GROCERY"},
		{CategoryMarket, "Groceries", "JARDIM SATELITE HORTIF", "MARKET_FRESH"},
		{CategoryMarket, "Groceries", "SWIFT JACAREI", "MARKET_FRESH"},
		{CategoryMarket, "Groceries", "CACAU SHOW", "MARKET_CONVENIENCE"},
		{CategoryTransport, "Tolls and in vehicle payment", "SEM*PARAR*ABASTECE (3/6)", "TRANSPORT_FUEL"},
		{CategoryTransport, "Tolls and in vehicle payment", "SEM PARAR", "TRANSPORT_TOLL"},
		{CategoryTransport, "Parking", "ESTAPAR MONO SEDE SANT", "TRANSPORT_PARKING"},
		{CategoryTransport, "Taxi and ride-hailing", "UBER * PENDING", "TRANSPORT_RIDE"},
		{CategoryTransport, "Taxi and ride-hailing", "99*", "TRANSPORT_RIDE"},
		{CategoryTransport, "Gas stations", "AUTO POSTO CENTRAL DE", "TRANSPORT_FUEL"},
		{CategoryTransport, "Vehicle maintenance", "DETAIL GARAGE JK SANTA (1/2)", "TRANSPORT_WASH"},
		{CategoryTransport, "Vehicle maintenance", "SM MECANICA", "TRANSPORT_MAINTENANCE"},
		{CategoryTransport, "Vehicle ownership taxes and fees", "Pagamento de conta DETRAN - SP", "TRANSPORT_FEES"},
		{CategoryTransport, "Public transportation", "PASSARO MARRON", "TRANSPORT_PUBLIC"},
		{CategoryTransport, "Automotive", "PIX ENVIADO SINAL PRIME DISTRI", "TRANSPORT_OTHER"},
		{CategoryHealth, "Pharmacy", "RAIA2073", "HEALTH_PHARMACY"},
		{CategoryHealth, "Gyms and fitness centers", "Wellhub Gympass BR", "HEALTH_GYM"},
		{CategoryHealth, "Healthcare", "UNHAS CARIOCAS SHOPPIN", "HEALTH_BEAUTY"},
		{CategoryEntertainment, "Digital services", "DM*Spotify", "ENTERTAINMENT_STREAMING"},
		{CategoryEntertainment, "Leisure", "NICK FUN DIVERSOES", "ENTERTAINMENT_OUTINGS"},
		{CategoryShopping, "Shopping", "VIVARA SJV (3/10)", "SHOPPING_BEAUTY"},
		{CategoryShopping, "Clothing", "Shein *SHEINCOM (6/6)", "SHOPPING_CLOTHING"},
		{CategoryShopping, "Shopping", "DL *Alipay MAGAZINE LU (8/12)", "SHOPPING_ONLINE"},
		{CategoryShopping, "Shopping", "LOJA QUALQUER", "SHOPPING_OTHER"},
		{CategoryBills, "Telecommunications", "CONTA VIVO", "BILLS_TELECOM"},
		{CategoryBills, "Utilities", "Pagamento de conta PORTOSEG SA C", "BILLS_INSURANCE"},
		{CategoryBills, "Loans and financing", "PRESTACAO CONSORCIO PGTO EVENT", "BILLS_LOANS"},
		{CategoryBills, "Electricity", "Pagamento de conta EDP SP", "BILLS_ELECTRICITY"},
		{CategoryBills, "Services", "DON GERONIMO BARBEARIA", "BILLS_OTHER"},
		{CategoryHousing, "Rent", "Pix enviado Imobiliaria Silva", "HOUSING_RENT"},
		{CategoryHousing, "", "Condominio", "HOUSING_CONDO"},
		{CategoryEducation, "Bookstore", "AMAZON BR", "EDUCATION_BOOKS"},
		{CategoryPeople, "", "Pix enviado Maria", ""},
		{CategoryOther, "", "qualquer coisa", ""},
	}
	for _, c := range cases {
		if got := Classify(c.category, c.pluggy, c.description); got != c.want {
			t.Errorf("Classify(%s, %q, %q) = %q, quer %q", c.category, c.pluggy, c.description, got, c.want)
		}
	}
}

func TestSubcategoryCatalog(t *testing.T) {
	seen := map[Subcategory]bool{}
	fallbacks := map[Category]int{}
	for _, s := range subcategories {
		if seen[s.info.Code] {
			t.Errorf("subcategoria repetida: %s", s.info.Code)
		}
		seen[s.info.Code] = true
		if !s.info.Category.IsKnown() || s.info.Label == "" {
			t.Errorf("subcategoria mal definida: %+v", s.info)
		}
		if s.keywords == nil && s.pluggy == nil {
			fallbacks[s.info.Category]++
		}
	}
	for c, n := range fallbacks {
		if n != 1 {
			t.Errorf("%s tem %d subcategorias que sobram; quer 1", c, n)
		}
	}
	for _, s := range subcategories {
		if fallbacks[s.info.Category] != 1 {
			t.Errorf("%s não tem subcategoria que sobra", s.info.Category)
		}
	}
	if p, _ := Subcategory("TRANSPORT_PARKING").Info(); p.Essential == nil || !*p.Essential {
		t.Error("estacionamento é essencial")
	}
	if !Subcategory("FOOD_DELIVERY").BelongsTo(CategoryFood) || Subcategory("FOOD_DELIVERY").BelongsTo(CategoryMarket) {
		t.Error("BelongsTo errado")
	}
}
