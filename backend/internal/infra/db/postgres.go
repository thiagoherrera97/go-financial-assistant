package db

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
	loc  atomic.Pointer[time.Location]
}

// NewPostgres abre o pool. loc é o fuso da sessão (DIGEST_TIMEZONE): ele decide o dia e o mês de tudo que é derivado de um
// instante no SQL (created_at::date, DATE_TRUNC sobre timestamptz). Os lançamentos manuais não têm vencimento, então
// um gasto das 22h do dia 30 em Brasília (01h UTC do dia 1) só cai no mês certo com a sessão no fuso do dono. nil mantém
// o fuso padrão do servidor (UTC na imagem).
func NewPostgres(ctx context.Context, databaseURL string, loc *time.Location) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear DATABASE_URL: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2

	d := &DB{}
	d.loc.Store(loc)
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return d.applyTimezone(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar pool postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("erro ao conectar no postgres: %w", err)
	}

	d.Pool = pool
	return d, nil
}

// applyTimezone põe o fuso configurado na sessão de uma conexão nova.
func (d *DB) applyTimezone(ctx context.Context, conn *pgx.Conn) error {
	loc := d.loc.Load()
	if loc == nil || loc.String() == "Local" {
		return nil
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('TimeZone', $1, false)`, loc.String()); err != nil {
		return fmt.Errorf("erro ao definir o fuso da sessão (%s): %w", loc, err)
	}
	return nil
}

// SetTimezone troca o fuso das sessões. O banco abre antes da configuração ser lida (as configurações salvas vêm dele),
// então o fuso chega depois: as conexões já abertas são descartadas e as novas nascem no fuso certo.
func (d *DB) SetTimezone(loc *time.Location) {
	d.loc.Store(loc)
	d.Pool.Reset()
}

func (d *DB) Close() {
	d.Pool.Close()
}
