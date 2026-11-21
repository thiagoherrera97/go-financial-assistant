// Package cycles mantém os ciclos do salário: calcula pelo salário configurado e pelos créditos que chegaram, grava na
// tabela que o SQL usa e guarda em memória para o lado Go perguntar "em que ciclo cai este dia".
package cycles

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// aheadYears é até onde os ciclos vão além de hoje (projeção, parcelas futuras).
const aheadYears = 2

// Credits lê os créditos de salário entre duas datas.
type Credits func(ctx context.Context, from, to time.Time) ([]domain.SalaryCredit, error)

// Service guarda os ciclos atuais. Current é seguro para uso concorrente.
type Service struct {
	store   ports.CycleStore
	credits Credits
	parts   func() []domain.SalaryPart
	now     func() time.Time

	mu      sync.Mutex // serializa Refresh
	loaded  bool
	last    []domain.CycleStart
	current atomic.Pointer[domain.Cycles]
}

func New(store ports.CycleStore, credits Credits, parts func() []domain.SalaryPart, now func() time.Time) *Service {
	return &Service{store: store, credits: credits, parts: parts, now: now}
}

// Current são os ciclos em uso; antes do primeiro Refresh, vazio (mês do calendário).
func (s *Service) Current() domain.Cycles {
	if c := s.current.Load(); c != nil {
		return *c
	}
	return domain.NewCycles(nil)
}

// Refresh recalcula os ciclos e regrava a tabela só se mudaram. Chamado ao subir, quando a configuração muda e depois de
// cada sincronização (um salário adiantado muda o início do ciclo).
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		prev, err := s.store.LoadCycles(ctx)
		if err != nil {
			return err
		}
		s.last, s.loaded = prev, true
		c := domain.NewCycles(prev)
		s.current.Store(&c)
	}
	starts, err := s.build(ctx)
	if err != nil {
		return err
	}
	if !slices.EqualFunc(starts, s.last, func(a, b domain.CycleStart) bool { return a.Key.Equal(b.Key) && a.Start.Equal(b.Start) }) {
		if err := s.store.ReplaceCycles(ctx, starts); err != nil {
			return err
		}
		s.last = starts
	}
	c := domain.NewCycles(starts)
	s.current.Store(&c)
	return nil
}

func (s *Service) build(ctx context.Context) ([]domain.CycleStart, error) {
	parts := s.parts()
	if len(parts) == 0 {
		return nil, nil
	}
	now := s.now()
	from := now
	if oldest, ok, err := s.store.OldestPaymentDate(ctx); err != nil {
		return nil, err
	} else if ok && oldest.Before(now) {
		from = oldest
	}
	to := now.AddDate(aheadYears, 0, 0)
	credits, err := s.credits(ctx, from.AddDate(0, 0, -domain.SalaryMatchDays), to)
	if err != nil {
		return nil, err
	}
	return domain.BuildCycles(parts, credits, from, to), nil
}
