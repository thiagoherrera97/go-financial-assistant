package insights

import (
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/balances"
	"github.com/google/uuid"
)

func TestCoverOverdraftHidesNegative(t *testing.T) {
	santander, other := uuid.New(), uuid.New()
	total := -1500.0 + 500
	view := balances.BalancesView{TotalInAccount: &total, Institutions: []balances.InstitutionBalance{
		{Name: "Santander", HasBank: true, Total: -1500, Accounts: []balances.BalanceAccount{{ID: santander.String(), Balance: -1500}}},
		{Name: "Inter", HasBank: true, Total: 500, Accounts: []balances.BalanceAccount{{ID: other.String(), Balance: 500}}},
	}}
	coverOverdraft(&view, []domain.Debt{{Kind: domain.DebtOverdraft, Name: "Ajuda do carro", AccountID: &santander}})

	if *view.TotalInAccount != 500 {
		t.Fatalf("total %v", *view.TotalInAccount)
	}
	var s balances.InstitutionBalance
	for _, in := range view.Institutions {
		if in.Name == "Santander" {
			s = in
		}
	}
	a := s.Accounts[0]
	if s.Total != 0 || a.Balance != 0 || a.Overdraft != 1500 || a.OverdraftDebt != "Ajuda do carro" {
		t.Fatalf("santander %+v", s)
	}
}

func TestCoverOverdraftKeepsUnlinkedNegative(t *testing.T) {
	id := uuid.New()
	total := -200.0
	view := balances.BalancesView{TotalInAccount: &total, Institutions: []balances.InstitutionBalance{
		{HasBank: true, Total: -200, Accounts: []balances.BalanceAccount{{ID: id.String(), Balance: -200}}},
	}}
	coverOverdraft(&view, nil)
	if *view.TotalInAccount != -200 || view.Institutions[0].Accounts[0].Balance != -200 {
		t.Fatal("conta sem dívida ligada deve continuar negativa")
	}
}
