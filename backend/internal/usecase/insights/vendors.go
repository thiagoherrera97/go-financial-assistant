package insights

import (
	"context"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"
)

// Vendors calcula o plano de fornecedores hoje: o guardado vem da caixinha escolhida (somada entre as contas
// correntes com o mesmo nome) e a sobra mensal, da projeção. Devolve também os nomes das caixinhas existentes.
func (i *Insights) Vendors(ctx context.Context, store ports.VendorStore, today time.Time) (planning.VendorSummary, []string, error) {
	plan, err := store.VendorPlan(ctx)
	if err != nil {
		return planning.VendorSummary{}, nil, err
	}
	vendors, err := store.Vendors(ctx)
	if err != nil {
		return planning.VendorSummary{}, nil, err
	}
	accounts, err := i.reader.Accounts(ctx)
	if err != nil {
		return planning.VendorSummary{}, nil, err
	}
	proj, err := i.Projection(ctx, MonthStart(today), goalProjectionMonths)
	if err != nil {
		return planning.VendorSummary{}, nil, err
	}

	boxes := map[string]float64{}
	var names []string
	for _, a := range accounts {
		if a.Type != domain.AccountBank {
			continue
		}
		for _, r := range a.Reserved {
			if _, seen := boxes[r.Name]; !seen {
				names = append(names, r.Name)
			}
			boxes[r.Name] += r.Amount
		}
	}
	slices.Sort(names)
	return planning.BuildVendorSummary(plan, vendors, boxes, proj, today), names, nil
}
