package domain

import "testing"

func TestTransferCounterparty(t *testing.T) {
	cases := map[string]string{
		"Pix enviado Carlos Eduardo Rocha":                                         "carlos eduardo rocha",
		"PIX RECEBIDO   Rafael Augusto Pereira Co":                                 "rafael augusto pereira co",
		"Transferência Pix recebida JOANA PRADO LIMA":                              "joana prado lima",
		"Transferência recebida Helena Duarte":                                     "helena duarte",
		"TED RECEBIDA Helena Duarte De Souza":                                      "helena duarte de souza",
		"Pix enviada Beatriz Ramos Teixeira":                                       "beatriz ramos teixeira",
		"Reembolso de transferência enviada EXEMPLO INSTITUICAO DE PAGAMENTO LTDA": "exemplo instituicao de pagamento ltda",
		"MP*CARLOSROCHA":   "",
		"DEP DINHEIRO ATM": "",
	}
	for in, want := range cases {
		if got := TransferCounterparty(in); got != want {
			t.Errorf("TransferCounterparty(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestLooksLikePersonName(t *testing.T) {
	people := []string{"carlos eduardo rocha", "helena duarte de souza", "rafael augusto pereira co", "beatriz ramos"}
	others := []string{"acme portal virtual s.a.", "xyz servicos medicos ltda me", "beta rent a car", "foopay instituicao de pagamento s.a.",
		"loja exemplo s.a - em recuperacao judicial", "maria", "joao 123", ""}
	for _, n := range people {
		if !LooksLikePersonName(n) {
			t.Errorf("%q deveria parecer pessoa", n)
		}
	}
	for _, n := range others {
		if LooksLikePersonName(n) {
			t.Errorf("%q não deveria parecer pessoa", n)
		}
	}
}

func TestAutoTransferDecision(t *testing.T) {
	memory := map[string]RememberedTransfer{
		TransferMemoryKey("Pix enviado Fernanda Souza da Costa Lima"): {Decision: TransferPayment, Category: "HOUSING"},
		TransferMemoryKey("Pix recebido ACME PORTAL VIRTUAL S.A."):    {Decision: TransferPayment, Category: "OTHER"},
	}
	// Lembra o que foi respondido, mesmo com o nome cortado por outro banco.
	if got, ok := AutoTransferDecision("PIX ENVIADO   Fernanda Souza da Costa", memory); !ok || got.Decision != TransferPayment || got.Category != "HOUSING" {
		t.Errorf("memória pelo nome: %+v %v", got, ok)
	}
	if got, ok := AutoTransferDecision("Pix recebido ACME PORTAL VIRTUAL S.A.", memory); !ok || got.Decision != TransferPayment {
		t.Errorf("empresa já respondida: %+v %v", got, ok)
	}
	// Nome de pessoa nunca visto: para uma pessoa.
	if got, ok := AutoTransferDecision("Pix recebido Helena Duarte De Souza", memory); !ok || got.Decision != TransferPerson {
		t.Errorf("pessoa: %+v %v", got, ok)
	}
	// Empresa nunca vista e descrição sem nome: continuam na fila.
	for _, d := range []string{"Transferência Pix recebida BETA RENT A CAR", "MP*CARLOSROCHA"} {
		if _, ok := AutoTransferDecision(d, memory); ok {
			t.Errorf("%q deveria ficar para a pessoa decidir", d)
		}
	}
}
