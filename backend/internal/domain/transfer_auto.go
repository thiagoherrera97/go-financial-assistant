package domain

import (
	"regexp"
	"strings"
)

// transferPrefix tira o começo das descrições de transferência dos bancos, sobrando só a outra ponta (o nome).
// Ex.: "Pix enviado Maria", "PIX RECEBIDO   Joao", "Transferência Pix recebida X", "TED RECEBIDA X".
var transferPrefix = regexp.MustCompile(`^(reembolso de )?(transferencia( pix)?|pix|ted|doc)( (enviad[oa]|recebid[oa]|cancelad[oa]))?\s+`)

// TransferCounterparty devolve o nome da outra ponta de uma transferência, normalizado com Fold, ou "" quando a descrição
// não segue o formato "Pix/TED/Transferência enviada|recebida <nome>".
func TransferCounterparty(description string) string {
	d := Fold(description)
	loc := transferPrefix.FindStringIndex(d)
	if loc == nil {
		return ""
	}
	return strings.TrimSpace(d[loc[1]:])
}

// companyWords marcam nome de empresa, instituição ou serviço: aí só a pessoa sabe se é pagamento, recebimento ou outra
// coisa, então a transferência continua na fila.
var companyWords = map[string]bool{
	"sa": true, "s.a": true, "s.a.": true, "s/a": true, "ltda": true, "ltda.": true, "me": true, "epp": true, "eireli": true,
	"mei": true, "ltd": true, "inc": true, "cia": true, "instituicao": true, "pagamento": true, "pagamentos": true,
	"banco": true, "bank": true, "servicos": true, "servico": true, "medicos": true, "comercio": true, "tecnologia": true,
	"portal": true, "virtual": true, "digital": true, "loja": true, "lojas": true, "store": true, "brasil": true,
	"recuperacao": true, "judicial": true, "holding": true, "seguros": true, "seguradora": true, "investimentos": true,
	"corretora": true, "associacao": true, "condominio": true, "igreja": true, "clube": true, "academia": true,
	"farmacia": true, "drogaria": true, "restaurante": true, "mercado": true, "supermercado": true, "posto": true,
	"grupo": true, "group": true, "solucoes": true, "consultoria": true, "sociedade": true, "cooperativa": true,
	"fundacao": true, "escola": true, "colegio": true, "universidade": true, "hospital": true, "clinica": true,
	"rent": true, "car": true, "transportes": true, "industria": true, "distribuidora": true, "atm": true,
}

// LooksLikePersonName diz se o nome parece de uma pessoa: duas palavras ou mais, só letras, sem marca de empresa.
func LooksLikePersonName(name string) bool {
	words := strings.Fields(name)
	if len(words) < 2 {
		return false
	}
	for _, w := range words {
		if companyWords[w] || strings.HasSuffix(w, "pay") {
			return false
		}
		for _, r := range w {
			if (r < 'a' || r > 'z') && r != '\'' {
				return false
			}
		}
	}
	return true
}

// TransferMemoryKey é a chave de "já respondi para este nome": os 20 primeiros caracteres do nome, porque um banco corta o
// nome no fim ("Rafael Augusto Pereira Co") e outro manda inteiro. Sem nome reconhecível, a descrição inteira.
func TransferMemoryKey(description string) string {
	key := TransferCounterparty(description)
	if key == "" {
		key = Fold(description)
	}
	if r := []rune(key); len(r) > 20 {
		key = string(r[:20])
	}
	return key
}

// RememberedTransfer é uma decisão já tomada para um nome (a categoria vale para PAYMENT).
type RememberedTransfer struct {
	Decision TransferDecision
	Category string
}

// AutoTransferDecision decide uma transferência sem perguntar: primeiro o que a pessoa já respondeu para o mesmo nome;
// senão, nome de pessoa vira "para uma pessoa". Nome de empresa, ou descrição sem nome, fica para a pessoa (ok = false).
// Os nomes da própria pessoa nem chegam aqui: a sincronização já os trata como entre contas.
func AutoTransferDecision(description string, memory map[string]RememberedTransfer) (RememberedTransfer, bool) {
	if m, ok := memory[TransferMemoryKey(description)]; ok {
		return m, true
	}
	if LooksLikePersonName(TransferCounterparty(description)) {
		return RememberedTransfer{Decision: TransferPerson}, true
	}
	return RememberedTransfer{}, false
}
