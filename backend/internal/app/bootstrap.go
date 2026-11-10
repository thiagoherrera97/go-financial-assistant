package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db/migrate"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/migrations"
)

// loadConfig abre o banco primeiro (as configurações salvas no dashboard valem mais que o ambiente) e então carrega a
// configuração. Uma configuração salva que deixe o app inválido nunca pode impedi-lo de subir (e de ser corrigida):
// nesse caso, vale só o ambiente.
func (a *app) loadConfig() error {
	databaseURL, secretKey, err := config.Bootstrap()
	if err != nil {
		return err
	}
	if a.db, err = db.NewPostgres(a.ctx, databaseURL); err != nil {
		return fmt.Errorf("banco: %w", err)
	}
	if err := a.applyMigrations(databaseURL); err != nil {
		return err
	}

	cipher, err := settings.NewCipher(secretKey)
	if err != nil {
		return fmt.Errorf("APP_SECRET_KEY inválida: %w", err)
	}
	a.store = db.NewSettingsStore(a.db)
	overrides, err := settings.LoadOverrides(a.ctx, a.store, cipher, a.logger)
	if err != nil {
		a.logger.Warn("configurações salvas no dashboard não carregadas (a migration 016 foi aplicada?)", "error", err)
	}

	if a.cfg, err = config.Load(overrides); err != nil {
		a.logger.Error("configuração salva no dashboard inválida; usando só o ambiente", "error", err)
		if a.cfg, err = config.Load(nil); err != nil {
			return err
		}
	}

	a.settings = settings.NewService(a.store, cipher, a.logger)
	if err := a.settings.Load(a.ctx); err != nil {
		a.logger.Warn("configurações do dashboard indisponíveis", "error", err)
	}
	a.onChange = func() <-chan struct{} {
		ch := make(chan struct{}, 1)
		a.settings.OnChange(func() {
			select {
			case ch <- struct{}{}:
			default:
			}
		})
		return ch
	}
	a.applyTimezone()
	a.loadSetup(cipher)
	return nil
}

// loadSetup lê o estado do setup pelo navegador. Aberto (sem senha ou sem canal, ou SETUP_REOPEN), o dashboard só
// serve o setup até ele terminar.
func (a *app) loadSetup(cipher *settings.Cipher) {
	a.channelConfigured.Store(a.cfg.ChannelConfigured())
	a.setup = setup.NewService(db.NewSetupStore(a.db), cipher, setup.Env{
		Token: a.cfg.SetupToken, Reopen: a.cfg.SetupReopen, EnvPassword: a.cfg.DashboardPassword, ChannelReady: a.channelConfigured.Load,
	}, a.logger)
	if err := a.setup.Load(a.ctx); err != nil {
		a.logger.Warn("dados do setup indisponíveis (a migration 019 foi aplicada?)", "error", err)
	}
	if !a.setup.Open() {
		return
	}
	if err := a.setup.TokenProblem(); err != nil {
		a.logger.Warn("setup pelo navegador aberto, mas ainda não pode começar", "motivo", err.Error())
		return
	}
	a.logger.Info("setup pelo navegador aberto: abra o dashboard e cole o token do .env (linha SETUP_TOKEN)", "reaberto", a.setup.Reopen())
}

// applyMigrations aplica as migrations pendentes, antes de qualquer outro uso do banco, com backup antes (pg_dump em
// MIGRATION_BACKUP_DIR, padrão ./backups). Falha numa migration interrompe a subida. Se só o backup falhar, nada é aplicado e o
// app sobe como antes: o erro fica no log e as telas das migrations faltando avisam, sem laço de reinício.
func (a *app) applyMigrations(databaseURL string) error {
	files, err := migrate.Load(migrations.FS)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	runner := &migrate.Runner{
		Pool:   a.db.Pool,
		Files:  files,
		Backup: migrate.PgDump{Dir: os.Getenv("MIGRATION_BACKUP_DIR"), URL: databaseURL, Logger: a.logger}.Run,
		Logger: a.logger,
	}
	res, err := runner.Run(a.ctx)
	switch {
	case errors.Is(err, migrate.ErrBackup):
		a.logger.Error("migrations pendentes NÃO foram aplicadas porque o backup antes delas falhou; o app sobe sem elas", "error", err)
	case err != nil:
		return fmt.Errorf("migrations: %w", err)
	case len(res.Applied) > 0:
		a.logger.Info("migrations aplicadas", "count", len(res.Applied), "backup", res.Backup)
	}
	return nil
}
