package domain

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Subcategory detalha a categoria de uma despesa (ex.: Alimentação › Delivery) e diz se o gasto é essencial ou supérfluo,
// para o planejamento mostrar o supérfluo que mais pesa. Cada subcategoria pertence a uma categoria só.
type Subcategory string

// SubcategoryInfo descreve uma subcategoria. Essential nil = depende do caso (o app pede para você revisar).
type SubcategoryInfo struct {
	Code      Subcategory
	Category  Category
	Label     string
	Essential *bool
}

var (
	yes = func() *bool { b := true; return &b }()
	no  = func() *bool { b := false; return &b }()
)

// subRule casa uma subcategoria pela descrição (trechos já normalizados: minúsculas, sem acento nem pontuação; espaço nas
// pontas vale como borda de palavra) ou pela categoria do Pluggy (inteira, minúscula).
type subRule struct {
	info     SubcategoryInfo
	keywords []string
	pluggy   []string
}

// subcategories está na ordem em que as regras são testadas dentro de cada categoria: a primeira que bater vence, primeiro
// pela descrição e só depois pela categoria do Pluggy. A última de cada categoria sem palavras é a que sobra (fallback).
var subcategories = []subRule{
	// Alimentação: delivery antes de tudo ("99Food *KFC" é delivery), refeição do dia a dia antes de restaurante.
	{SubcategoryInfo{"FOOD_DELIVERY", CategoryFood, "Delivery", no},
		[]string{"ifood", " ifd ", "rappi", "uber eats", "99food", "99 food", "keeta", "aiqfome", "delivery", "james "},
		[]string{"food delivery"}},
	{SubcategoryInfo{"FOOD_DAILY", CategoryFood, "Refeição do dia a dia", yes},
		[]string{"marmit", "self service", " quilo ", " kilo ", "por quilo", "refeicao", "refeicoes", "prato feito", "comida caseira", "restaurante universitario"},
		nil},
	{SubcategoryInfo{"FOOD_BAKERY", CategoryFood, "Padaria", yes},
		[]string{"padaria", "panificadora", "panificacao", "bakery"},
		[]string{"bakery"}},
	{SubcategoryInfo{"FOOD_SNACK", CategoryFood, "Lanche, café e doces", no},
		[]string{" cafe ", "cafeteria", "coffee", "starbucks", "lanche", "lanchonete", "pastel", "sorvete", "acai", "burger", "burguer",
			"mcdonald", "burger king", " bk ", "popeyes", " kfc ", "subway", "habib", "doceria", "confeitaria", "casa de suc", " suco",
			"cacau show", "kopenhagen", "chocolate", "espetinho", "hot dog", "pipoca", "donuts", "brigadeir", "fast "},
		nil},
	{SubcategoryInfo{"FOOD_RESTAURANT", CategoryFood, "Restaurante e bar", no},
		[]string{"restaurante", "restaurant", "restau", "grill", "steakhouse", "outback", "sushi", "temaki", "pizza", "churrascaria",
			" bar ", "choperia", "chopp", "cervejaria", " pub ", "boteco", "bistro", "cantina", "trattoria", "piadina", "japa"},
		[]string{"eating out", "restaurants", "food and drinks", "bars"}},
	{SubcategoryInfo{"FOOD_OTHER", CategoryFood, "Outros (Alimentação)", nil}, nil, nil},

	// Mercado: o que não for conveniência nem feira é supermercado.
	{SubcategoryInfo{"MARKET_CONVENIENCE", CategoryMarket, "Conveniência e doces", no},
		[]string{"conveniencia", " am pm ", "ampm", "cacau show", "kopenhagen", "chocolate", "nutty bavarian", "bombom", "doceria"},
		nil},
	{SubcategoryInfo{"MARKET_FRESH", CategoryMarket, "Feira, hortifrúti e açougue", yes},
		[]string{"hortifruti", "hortif", "sacolao", " feira ", "acougue", "casa de carne", "peixaria", "swift "},
		nil},
	{SubcategoryInfo{"MARKET_GROCERY", CategoryMarket, "Supermercado", yes}, nil, nil},

	// Transporte: combustível antes de pedágio ("SEM PARAR ABASTECE" é posto pago pela tag).
	{SubcategoryInfo{"TRANSPORT_FUEL", CategoryTransport, "Combustível", yes},
		[]string{" posto ", "auto posto", "combustivel", "abastece", "gasolina", "etanol", "ipiranga", "petrobras", " shell "},
		[]string{"gas stations", "fuel"}},
	{SubcategoryInfo{"TRANSPORT_RIDE", CategoryTransport, "Aplicativo (Uber, 99)", no},
		[]string{"uber", "99 pop", "99app", " 99 ", "cabify", "indriver", "indrive", " taxi "},
		[]string{"taxi and ride-hailing"}},
	{SubcategoryInfo{"TRANSPORT_WASH", CategoryTransport, "Lava-rápido e estética automotiva", no},
		[]string{"lava rapido", "lava jato", "lavajato", "lava car", "lavacar", "lava auto", "estetica automotiva", "detail", "polimento", "lavagem"},
		nil},
	{SubcategoryInfo{"TRANSPORT_PARKING", CategoryTransport, "Estacionamento", yes},
		[]string{"estapar", "estacionamento", "parking", "zona azul", "multipark", "indigo", " park "},
		[]string{"parking"}},
	{SubcategoryInfo{"TRANSPORT_TOLL", CategoryTransport, "Pedágio e tag", yes},
		[]string{"sem parar", "conectcar", "veloe", "move mais", "pedagio", "taggy"},
		[]string{"tolls and in vehicle payment"}},
	{SubcategoryInfo{"TRANSPORT_PUBLIC", CategoryTransport, "Transporte público e ônibus", yes},
		[]string{" metro ", "cptm", "sptrans", "bilhete unico", "onibus", "passaro marron", "rodoviaria", "buser", "viacao"},
		[]string{"public transportation"}},
	{SubcategoryInfo{"TRANSPORT_MAINTENANCE", CategoryTransport, "Manutenção e peças", yes},
		[]string{"mecanica", "mecanico", "auto pecas", "autopecas", " pneu", "borracharia", "bateria", "oficina", "automotivo",
			"funilaria", "insulfilm", "troca de oleo", "alinhamento"},
		[]string{"vehicle maintenance"}},
	{SubcategoryInfo{"TRANSPORT_FEES", CategoryTransport, "IPVA, licenciamento e seguro", yes},
		[]string{"detran", "ipva", "licenciamento", "vistoria", "seguro auto", " multa"},
		[]string{"vehicle ownership taxes and fees"}},
	{SubcategoryInfo{"TRANSPORT_OTHER", CategoryTransport, "Outros (Transporte)", nil}, nil, nil},

	// Saúde.
	{SubcategoryInfo{"HEALTH_GYM", CategoryHealth, "Academia e esportes", no},
		[]string{"wellhub", "gympass", "smart fit", "smartfit", "academia", "bodytech", "crossfit", "totalpass", "bluefit"},
		[]string{"gyms and fitness centers", "wellness and fitness"}},
	{SubcategoryInfo{"HEALTH_BEAUTY", CategoryHealth, "Estética e beleza", no},
		[]string{"unhas", "manicure", " salao ", "cabeleireir", "barbearia", "estetica", "depilacao", " spa "},
		nil},
	{SubcategoryInfo{"HEALTH_PHARMACY", CategoryHealth, "Farmácia", yes},
		[]string{"drogaria", "droga", "drogasil", " raia", "farmacia", "pague menos", "panvel", "farmaconde", "rd saude", "pacheco"},
		[]string{"pharmacy"}},
	{SubcategoryInfo{"HEALTH_PLAN", CategoryHealth, "Plano de saúde", yes},
		[]string{"unimed", " amil", "bradesco saude", "sulamerica", "hapvida", "notredame", "plano de saude"},
		[]string{"health insurance"}},
	{SubcategoryInfo{"HEALTH_CARE", CategoryHealth, "Consultas, exames e terapia", yes},
		[]string{"clinica", "hospital", "laboratorio", "odonto", "dentista", "consulta", "medic", "psicolog", "fisioterap", "exame",
			"santa casa", "fleury", "lavoisier", "delboni"},
		[]string{"healthcare", "medical", "dental", "hospitals and clinics"}},
	{SubcategoryInfo{"HEALTH_OTHER", CategoryHealth, "Outros (Saúde)", yes}, nil, nil},

	// Lazer: tudo supérfluo.
	{SubcategoryInfo{"ENTERTAINMENT_STREAMING", CategoryEntertainment, "Streaming e assinaturas", no},
		[]string{"netflix", "spotify", "youtube", "prime video", "amazon prime", "disney", " hbo", " max ", "globoplay", "deezer",
			"apple com", "paramount", "crunchyroll", "melimais", "meli "},
		[]string{"digital services", "streaming"}},
	{SubcategoryInfo{"ENTERTAINMENT_GAMES", CategoryEntertainment, "Jogos e apostas", no},
		[]string{"steam", "playstation", " psn ", "xbox", "nintendo", " bet ", "bet365", "aposta", "blaze", "epic games", "riot"},
		[]string{"gaming", "online bet"}},
	{SubcategoryInfo{"ENTERTAINMENT_TRAVEL", CategoryEntertainment, "Viagem", no},
		[]string{"hotel", "pousada", "airbnb", "booking", "latam", " gol ", "azul linhas", "decolar", "hurb", "123milhas", "resort"},
		[]string{"travel", "accomodation", "accommodation", "airlines"}},
	{SubcategoryInfo{"ENTERTAINMENT_OUTINGS", CategoryEntertainment, "Passeios e eventos", no},
		[]string{"cinema", "cinemark", "ingresso", "bilhete", "sympla", "eventim", " show", "teatro", " parque", "diversoes", " fun "},
		[]string{"leisure", "cinema, theater and concerts", "stadiums and arenas", "tickets"}},
	{SubcategoryInfo{"ENTERTAINMENT_OTHER", CategoryEntertainment, "Outros (Lazer)", no}, nil, nil},

	// Compras: Shein é roupa, mesmo sendo on-line; o resto do marketplace é compra on-line.
	{SubcategoryInfo{"SHOPPING_GIFTS", CategoryShopping, "Presentes", no},
		[]string{"presente", "floricultura", " flores", "brinquedo", " toys", "pelucia", "ri happy"},
		[]string{"kids and toys", "gifts"}},
	{SubcategoryInfo{"SHOPPING_BEAUTY", CategoryShopping, "Joias, cosméticos e acessórios", no},
		[]string{"vivara", "pandora", "sephora", "boticario", "natura", " avon", "joalheria", "relojoaria", "cabeleireiro"},
		nil},
	{SubcategoryInfo{"SHOPPING_ELECTRONICS", CategoryShopping, "Eletrônicos", no},
		[]string{"kabum", "fast shop", "fastshop", "apple store", "samsung", "eletronic", "informatica", "pichau", "terabyte"},
		[]string{"electronics"}},
	{SubcategoryInfo{"SHOPPING_CLOTHING", CategoryShopping, "Roupas e calçados", yes},
		[]string{"renner", "riachuelo", " cea ", "hering", " zara", "shein", "centauro", "netshoes", "calcado", "marisa",
			"pernambucanas", "youcom", "calvin klein", "tommy", "constance", "arezzo", " nike", "adidas", "decathlon"},
		[]string{"clothing", "sports goods"}},
	{SubcategoryInfo{"SHOPPING_HOME", CategoryShopping, "Casa e utilidades", yes},
		[]string{"leroy", "tok stok", "tokstok", "camicado", "havan", "utilidades", "ferragens", "material de constru", "telhanorte", "kalunga"},
		[]string{"houseware", "home improvement", "pet supplies and vet"}},
	{SubcategoryInfo{"SHOPPING_ONLINE", CategoryShopping, "Compras on-line", no},
		[]string{"mercadolivre", "mercado livre", "shopee", "shpp", "aliexpress", "alipay", "amazon", "magalu", "magazine lu", "temu", "americanas"},
		[]string{"online shopping"}},
	{SubcategoryInfo{"SHOPPING_OTHER", CategoryShopping, "Outras compras", no}, nil, nil},

	// Moradia: essencial.
	{SubcategoryInfo{"HOUSING_RENT", CategoryHousing, "Aluguel e financiamento", yes},
		[]string{"aluguel", "imobiliaria", "quintoandar", "quinto andar", "habitacional"},
		[]string{"rent"}},
	{SubcategoryInfo{"HOUSING_CONDO", CategoryHousing, "Condomínio", yes}, []string{"condominio", " cond "}, nil},
	{SubcategoryInfo{"HOUSING_TAX", CategoryHousing, "IPTU", yes}, []string{"iptu"}, nil},
	{SubcategoryInfo{"HOUSING_OTHER", CategoryHousing, "Outros (Moradia)", yes}, nil, nil},

	// Contas da casa: essencial. O que o Pluggy chama só de "services" (barbearia, lavanderia...) fica para você revisar.
	{SubcategoryInfo{"BILLS_ELECTRICITY", CategoryBills, "Luz", yes},
		[]string{" enel", "cpfl", " edp ", " light ", "cemig", "copel", "energisa", "coelba", "celesc", "eletropaulo", "neoenergia", "equatorial", "energia eletrica"},
		[]string{"electricity"}},
	{SubcategoryInfo{"BILLS_WATER", CategoryBills, "Água", yes},
		[]string{"sabesp", "sanasa", " saae", "copasa", "sanepar", "embasa", "cedae", "aegea", " agua "},
		[]string{"water"}},
	{SubcategoryInfo{"BILLS_GAS", CategoryBills, "Gás", yes},
		[]string{"comgas", "naturgy", "ultragaz", "liquigas", "supergasbras", "copagaz", " gas "},
		[]string{"gas"}},
	{SubcategoryInfo{"BILLS_TELECOM", CategoryBills, "Internet, TV e celular", yes},
		[]string{"vivo", "claro", " tim ", " oi ", " net ", " sky ", "algar", "fibra", "internet", "telecom", "brisanet"},
		[]string{"telecommunications", "internet", "mobile", "tv"}},
	{SubcategoryInfo{"BILLS_INSURANCE", CategoryBills, "Seguros", yes},
		[]string{"seguro", "portoseg", "porto seguro", "mapfre", "allianz", "tokio marine", " hdi "},
		[]string{"life insurance", "insurance"}},
	{SubcategoryInfo{"BILLS_LOANS", CategoryBills, "Empréstimos e consórcios", yes},
		[]string{"consorcio", "emprestimo", "financiamento", "prestacao", " prest "},
		[]string{"loans and financing", "financing"}},
	{SubcategoryInfo{"BILLS_OTHER", CategoryBills, "Outras contas e serviços", nil}, nil, nil},

	// Educação: essencial.
	{SubcategoryInfo{"EDUCATION_COURSES", CategoryEducation, "Cursos, escola e faculdade", yes},
		[]string{"faculdade", "universidade", "curso", "escola", "colegio", "udemy", "alura", "mensalidade"},
		[]string{"university", "school", "education"}},
	{SubcategoryInfo{"EDUCATION_BOOKS", CategoryEducation, "Livros e material", yes},
		[]string{"livraria", "kindle", "papelaria", "amazon"},
		[]string{"bookstore"}},
	{SubcategoryInfo{"EDUCATION_OTHER", CategoryEducation, "Outros (Educação)", yes}, nil, nil},
}

var subcategoryIndex = func() map[Subcategory]int {
	m := make(map[Subcategory]int, len(subcategories))
	for i, s := range subcategories {
		m[s.info.Code] = i
	}
	return m
}()

// Subcategories devolve o catálogo na ordem de exibição (agrupado por categoria).
func Subcategories() []SubcategoryInfo {
	out := make([]SubcategoryInfo, len(subcategories))
	for i, s := range subcategories {
		out[i] = s.info
	}
	return out
}

// Info devolve a descrição da subcategoria; ok falso se o código não existe.
func (s Subcategory) Info() (SubcategoryInfo, bool) {
	i, ok := subcategoryIndex[s]
	if !ok {
		return SubcategoryInfo{}, false
	}
	return subcategories[i].info, true
}

// Label é o nome da subcategoria; vazio se não existe.
func (s Subcategory) Label() string {
	info, _ := s.Info()
	return info.Label
}

// BelongsTo diz se a subcategoria existe e é da categoria c.
func (s Subcategory) BelongsTo(c Category) bool {
	info, ok := s.Info()
	return ok && info.Category == c
}

// HasSubcategories diz se a categoria se divide em subcategorias (Pessoas e Outros não se dividem).
func (c Category) HasSubcategories() bool {
	for _, s := range subcategories {
		if s.info.Category == c {
			return true
		}
	}
	return false
}

// Classify escolhe a subcategoria de uma despesa pela descrição e pela categoria do Pluggy. Categoria sem subcategorias
// devolve "". A descrição vence a categoria do Pluggy, que vence a subcategoria que sobra.
func Classify(category Category, pluggyCategory, description string) Subcategory {
	text := normalizeForMatch(description)
	pluggy := strings.ToLower(strings.TrimSpace(pluggyCategory))
	var fallback Subcategory
	for _, s := range subcategories {
		if s.info.Category != category {
			continue
		}
		for _, k := range s.keywords {
			if strings.Contains(text, k) {
				return s.info.Code
			}
		}
		if s.keywords == nil && s.pluggy == nil {
			fallback = s.info.Code
		}
	}
	if pluggy != "" {
		for _, s := range subcategories {
			if s.info.Category != category {
				continue
			}
			for _, p := range s.pluggy {
				if pluggy == p {
					return s.info.Code
				}
			}
		}
	}
	return fallback
}

// normalizeForMatch deixa a descrição comparável às palavras-chave: minúsculas, sem acento, só letras e números separados
// por um espaço, com um espaço em cada ponta.
func normalizeForMatch(s string) string {
	var b strings.Builder
	b.WriteByte(' ')
	space := true
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		case !space:
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	return b.String()
}
