package app

import (
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/openfinance"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/logo"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
)

// startSync liga o Open Finance, que se liga e desliga com o app rodando: sem credenciais a sincronização apenas espera.
func (a *app) startSync() error {
	client := pluggy.NewClient(a.cfg.PluggyClientID, a.cfg.PluggyClientSecret)
	a.syncer = openfinance.NewSyncOpenFinance(a.ledger, client, nil, a.cfg.OpenFinanceLookbackDays, a.logger)
	a.syncer.SetLogoFetcher(logo.New())
	a.syncer.SetCategoryMapping(pluggy.ExactExpenseCategories())
	a.syncer.SetLocation(a.cfg.DigestLocation)
	a.subcategories = db.NewDashboardRepo(a.db)
	go a.runSync(client, a.onChange())
	return nil
}

// runSync sincroniza quando o intervalo vence, relendo a configuração a cada minuto e quando ela muda: credenciais, bancos,
// janela de datas e nomes próprios valem sem reiniciar.
func (a *app) runSync(client *pluggy.Client, changed <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	var (
		last      time.Time
		lastCreds string
		lastClose int
		announced bool
	)
	for {
		s := a.settings.Sync()
		client.SetOwnNames(a.settings.List("OWN_NAMES"))
		if creds := s.ClientID + "\x00" + s.ClientSecret; creds != lastCreds {
			client.SetCredentials(s.ClientID, s.ClientSecret)
			lastCreds = creds
		}
		a.syncer.SetConfig(s.ItemIDs, s.LookbackDays)
		a.syncer.SetCardCloseDays(s.CardCloseDays)
		if lastClose != 0 && s.CardCloseDays != lastClose {
			// Mudou o fechamento: recalcula o mês das compras já sincronizadas sem esperar a próxima sincronização.
			if err := a.syncer.RecomputeCardBills(a.ctx); err != nil {
				a.logger.Error("erro ao recalcular a fatura das compras no cartão", "error", err)
			}
		}
		lastClose = s.CardCloseDays

		if s.Configured() {
			if !announced {
				a.logger.Info("Open Finance ativo", "items", len(s.ItemIDs), "interval", s.Interval.String())
				announced = true
			}
			if last.IsZero() || time.Since(last) >= s.Interval {
				result, err := a.syncer.Sync(a.ctx)
				last = time.Now()
				if err != nil {
					a.logger.Error("erro ao sincronizar Open Finance", "error", err)
				}
				a.logger.Info("sincronização do Open Finance concluída",
					"inserted", result.Inserted, "reconciled", result.Reconciled, "existing", result.Existing)
			}
		} else {
			announced = false
		}

		// A cada volta os ciclos acompanham o salário configurado e os créditos que chegaram (regrava só se mudou).
		if err := a.cycles.Refresh(a.ctx); err != nil {
			a.logger.Error("erro ao recalcular os ciclos do salário", "error", err)
		}

		// A cada volta (e logo ao subir) a subcategoria acompanha o que a sincronização, as regras e as correções mudaram.
		if n, err := a.subcategories.RefreshSubcategories(a.ctx); err != nil {
			a.logger.Error("erro ao preencher subcategorias", "error", err)
		} else if n > 0 {
			a.logger.Info("subcategorias atualizadas", "changed", n)
		}

		select {
		case <-a.ctx.Done():
			return
		case <-changed:
		case <-ticker.C:
		}
	}
}
