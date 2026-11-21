# Ciclo do salário — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Trocar o "mês do calendário" pelo ciclo do salário (hoje 15 → 14) em todo o app, mantendo `AAAA-MM` como chave.

**Architecture:** O Go calcula o início de cada ciclo (dia útil do salário ou crédito real adiantado) e grava numa tabela
`cycles`. O SQL atribui cada lançamento a um ciclo por uma função `cycle_of(date)` (que cai no mês do calendário quando a
tabela está vazia). O lado Go pergunta a um único `domain.Cycles` qual a chave de uma data e o início/fim de uma chave.
Como a chave continua `AAAA-MM-01`, iterações "mês a mês", orçamento, metas e contratos da API não mudam de forma.

**Tech Stack:** Go, PostgreSQL (migrations em `backend/migrations`), React + TS (`frontend/src`).

**Spec:** `docs/superpowers/specs/2026-11-21-ciclo-do-salario-design.md`

## Global Constraints

- Chave do ciclo = `AAAA-MM-01` do mês do dia **combinado** da 1ª parte de `SALARY_SCHEDULE`.
- Início = `domain.SalaryPayDate(ano, mês, parts[0].Day)`; se o crédito que `SalaryMonth` casa com a parte 0 chegou antes, o dia desse crédito.
- Fim = véspera do início do ciclo seguinte.
- Sem salário configurado: ciclo = mês do calendário.
- Lançamento sem `bill_due_date` e sem `due_date`, com `reference_month`: cai na chave `DATE_TRUNC('month', reference_month)`.
- Cartão: pela data efetiva de hoje (`bill_due_date` primeiro); a atribuição compra → fatura não muda.
- Dívidas (`usecase/debts`, `insights/debts.go`) continuam em datas reais de vencimento; só a "chave atual" delas passa a vir do ciclo.
- Textos da interface em português; "Mês" vira "Ciclo" onde o período é o ciclo.
- Commit + push a cada tarefa (CLAUDE.md), mensagens em português, Conventional Commits, com `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Salário num feriado/fim de semana:** 15/nov/2026 é domingo e feriado → o ciclo de novembro começa sexta 13/nov. Teste em Task 1.
2. **Salário adiantado:** crédito da 1ª parte em 11/dez/2026, sexta (previsto 15/dez, terça) → ciclo de dezembro começa 11/dez e o ciclo de novembro termina 10/dez. Teste em Task 1.
3. **Tabela vazia / data antes do primeiro ciclo:** `cycle_of` devolve o mês do calendário. Teste em Task 2.
4. **Recorrente com só `reference_month` = 2026-11-01:** cai no ciclo de novembro, não no de outubro. Teste em Task 2.
5. **Troca do `SALARY_SCHEDULE` em tempo de execução:** a tabela é regravada sem reiniciar. Teste em Task 3.

---

### Task 1: `domain.Cycles`

**Files:**
- Create: `backend/internal/domain/cycle.go`
- Test: `backend/internal/domain/cycle_test.go`

**Interfaces:**
- Produces:
  - `type CycleStart struct { Key, Start time.Time }` (datas em UTC, meia-noite)
  - `func BuildCycles(parts []SalaryPart, credits []SalaryCredit, from, to time.Time) []CycleStart`: uma entrada por chave de `MonthStart(from)` até `MonthStart(to)`, ordenadas; `nil` sem partes.
  - `type Cycles struct{ /* starts ordenados */ }`, `func NewCycles(starts []CycleStart) Cycles`
  - `func (c Cycles) KeyOf(day time.Time) time.Time`: maior `Key` com `Start <= day`; sem entrada que sirva, o 1º dia do mês de `day`.
  - `func (c Cycles) Bounds(key time.Time) (start, end time.Time)`: `end` = véspera do início da chave seguinte; chave fora da tabela → mês do calendário.
  - `func (c Cycles) DayOf(day time.Time) (n, total int)`: posição de `day` no ciclo (1-based) e tamanho do ciclo, para os ritmos.

- [ ] **Step 1: testes que falham** em `cycle_test.go` (usa `ymd` de `salary_test.go`):
  - `TestBuildCyclesHolidayStart`: parts `15=5200;30=4366,62`, sem créditos, de 2026-11 a 2026-12 → `[{2026-11-01, 2026-11-13}, {2026-12-01, 2026-12-15}]`.
  - `TestBuildCyclesEarlyCredit`: o mesmo, com crédito Salário de 5200 em 2026-12-11 → o início de dezembro é `2026-12-11`.
  - `TestBuildCyclesNoSalary`: parts vazias → `nil`.
  - `TestCyclesKeyOf`: com as entradas do 1º teste, `KeyOf(2026-12-14)` = 2026-11-01, `KeyOf(2026-12-15)` = 2026-12-01, `KeyOf(2026-11-12)` = 2026-11-01 (antes do 1º início, cai no calendário), `NewCycles(nil).KeyOf(2026-12-14)` = 2026-12-01.
  - `TestCyclesBounds`: `Bounds(2026-11-01)` = (2026-11-13, 2026-12-14); `NewCycles(nil).Bounds(2026-11-01)` = (2026-11-01, 2026-11-30).
  - `TestCyclesDayOf`: `DayOf(2026-11-13)` = (1, 32).
- [ ] **Step 2:** `cd backend && go test ./internal/domain -run Cycle` → FAIL (símbolos indefinidos).
- [ ] **Step 3: implementar.** Para cada chave: `nominal := SalaryPayDate(key.Year(), key.Month(), parts[0].Day)`; `status, _ := SalaryMonth(parts, key, credits)`; se `status[0].Received != nil` e a data do crédito for antes de `nominal`, o início é a data do crédito. A última chave de `Bounds` sem sucessora termina no fim do mês do calendário seguinte ao início − 1 dia (use `nominal` do mês seguinte calculado com `SalaryPayDate`, guardando as `parts` em `Cycles`).
- [ ] **Step 4:** `go test ./internal/domain` → PASS.
- [ ] **Step 5:** commit `feat: ciclo do salário no domínio`.

### Task 2: tabela `cycles` e atribuição no SQL

**Files:**
- Create: `backend/migrations/034_create_cycles.sql`
- Create: `backend/internal/infra/db/cycles_repository.go`, test `cycles_repository_test.go` (mesmo padrão de `testrepo_test.go`)
- Modify: `backend/internal/infra/db/queries_common.go` (`paymentMonth`), `backend/internal/infra/db/portfolio_queries.go:54,62,67`
- Modify: `backend/internal/domain/ports/` (novo `CycleStore` em `ports/readers.go`)

**Interfaces:**
- Consumes: `domain.CycleStart`.
- Produces:
  - SQL: tabela `cycles(month date PRIMARY KEY, starts_on date NOT NULL UNIQUE)`; função `cycle_of(d date) RETURNS date` (STABLE: `month` da maior `starts_on <= d`, senão `date_trunc('month', d)::date`); função `cycle_next_start(key date) RETURNS date` (`starts_on` da chave `key + 1 month`, senão `key + 1 month`).
  - `ports.CycleStore { ReplaceCycles(ctx, []domain.CycleStart) error; LoadCycles(ctx) ([]domain.CycleStart, error); OldestPaymentDate(ctx) (time.Time, bool, error) }`, implementado por `*db.CycleRepo` (`db.NewCycleRepo(*Postgres)`).
  - `paymentMonth` passa a ser: `CASE WHEN pay.bill_due_date IS NULL AND pay.due_date IS NULL AND pay.reference_month IS NOT NULL THEN DATE_TRUNC('month', pay.reference_month)::date ELSE cycle_of(COALESCE(pay.bill_due_date, pay.due_date, pay.paid_at, pay.created_at)::date) END`. Manter o nome da constante (são 41 usos); atualizar o comentário.

- [ ] **Step 1: testes que falham** em `cycles_repository_test.go`:
  - `TestReplaceCyclesRoundTrip`: grava 2 entradas e lê as mesmas; gravar de novo com 1 entrada deixa só 1 (troca numa transação).
  - `TestCycleOfFallback`: tabela vazia → `SELECT cycle_of('2026-12-14')` = 2026-12-01.
  - `TestCycleOfWithCycles`: com `{2026-11-01, 2026-11-13}, {2026-12-01, 2026-12-15}`: `cycle_of('2026-12-14')` = 2026-11-01; `cycle_of('2026-11-12')` = 2026-11-01 (fallback); `cycle_next_start('2026-11-01')` = 2026-12-15.
  - `TestPaymentMonthReferenceMonth`: com as mesmas entradas, um pagamento só com `reference_month = 2026-11-01` sai em `MonthlyTotals` na chave 2026-11; um Pix em `paid_at = 2026-12-10` também; uma compra no cartão com `bill_due_date = 2027-01-10` sai na chave 2026-12.
- [ ] **Step 2:** `go test ./internal/infra/db -run 'Cycle|PaymentMonthReference'` → FAIL.
- [ ] **Step 3:** migration, repositório e troca de `paymentMonth`. Em `portfolio_queries.go`, trocar `s.month + INTERVAL '1 month'` por `cycle_next_start(s.month::date)`.
- [ ] **Step 4:** `go test ./internal/infra/db/...` → PASS, **inclusive os testes que já existiam** (tabela vazia = comportamento antigo).
- [ ] **Step 5:** commit `feat: lançamentos atribuídos ao ciclo do salário no banco`.

### Task 3: serviço de ciclos e quando regravar

**Files:**
- Create: `backend/internal/usecase/cycles/cycles.go`, test `cycles_test.go`
- Modify: `backend/internal/app/app.go` (`buildServices`), `backend/internal/app/sync.go` (depois de `a.syncer.Sync` com sucesso)
- Modify: `backend/internal/usecase/architecture_test.go` se ele listar pacotes permitidos

**Interfaces:**
- Consumes: `ports.CycleStore`, `domain.BuildCycles`, `domain.NewCycles`, leitor de `SalaryCredits(ctx, from, to)` (o mesmo de `ports.InsightsReader`).
- Produces:
  - `cycles.New(store ports.CycleStore, credits func(ctx, from, to time.Time) ([]domain.SalaryCredit, error), parts func() []domain.SalaryPart, now func() time.Time) *Service`
  - `(*Service).Refresh(ctx) error`: janela de `OldestPaymentDate` (ou `now`) até `now + 2 anos`; grava com `ReplaceCycles` e guarda o `domain.Cycles` em memória (`atomic.Pointer`).
  - `(*Service).Current() domain.Cycles` (seguro para concorrência; antes do 1º `Refresh`, ciclos vazios = calendário).
  - `app.cycles *cycles.Service`, chamado no boot (depois das migrations), no `settings.OnChange` **só quando `SALARY_SCHEDULE` mudou** (compara o texto anterior) e depois de cada sincronização.

- [ ] **Step 1: testes que falham** com store e créditos falsos: `TestRefreshWritesCycles` (parts 15/30, now = 2026-11-21 → `Current().KeyOf(2026-12-14)` = 2026-11-01 e o store recebeu as entradas); `TestRefreshNoSalaryClears` (parts vazias → store recebe lista vazia, `Current().KeyOf(2026-12-14)` = 2026-12-01).
- [ ] **Step 2:** `go test ./internal/usecase/cycles` → FAIL.
- [ ] **Step 3:** implementar e ligar em `app`.
- [ ] **Step 4:** `go test ./...` e `go build ./...` → PASS.
- [ ] **Step 5:** commit `feat: ciclos regravados no boot, ao mudar o salário e após sincronizar`.

### Task 4: lado Go usa o ciclo para "agora" e para intervalos de dias

**Files (modify):**
- `backend/internal/usecase/insights/insights.go` (`Salary`: janela de créditos = `Bounds(key)` ± `SalaryMatchDays`; o resto das funções que recebem `today` usa `KeyOf(today)`)
- `backend/internal/infra/http/params.go:43`, `goals_api.go:33`, `budget_api.go:82`, `summary_api.go`, `month_api.go` (chave atual = `cycles.Current().KeyOf(a.now())`)
- `backend/internal/usecase/planning/goals.go:69-70` (`DayOfMonth`/`DaysInMonth` ← `DayOf`), `planning/vendors.go:136`
- `backend/internal/usecase/ledger/analyze_expense.go:194`, `monthly_report.go:69`
- `backend/internal/infra/http/api.go` (campo `cycles func() domain.Cycles`) e o ponto onde `api` é montado

**Interfaces:**
- Consumes: `domain.Cycles` via `func() domain.Cycles` (= `app.cycles.Current`), injetado por setters no padrão de `SetSalarySchedule`: `Insights.SetCycles`, `AnalyzeExpense.SetCycles`, e no `api`.
- Produces: nenhum símbolo novo além dos setters. `insights.MonthStart` continua existindo para quem precisa do calendário (dívidas).
- Fora: `process_income.go:91` e `process_recurring.go:47` geram `reference_month` dos recorrentes. Ficam no calendário, porque a chave do mês N já é o ciclo N.

- [ ] **Step 1: testes que falham:** em `planning/goals_test.go`, `TestGoalProgressCyclePace` (ciclo 13/nov–14/dez, hoje 2026-11-21 → `DayOfMonth` 9, `DaysInMonth` 32); em `insights` (com reader falso), `TestSalaryUsesCycleWindow` (pede os créditos de 2026-11-08 a 2026-12-19 para a chave de novembro); em `http/month_api_test.go`, `TestCurrentMonthIsCycle` (now = 2026-12-14, `GET /api/transactions` sem `month` usa 2026-11).
- [ ] **Step 2:** `go test ./internal/...` → os 3 novos FAIL.
- [ ] **Step 3:** fazer as trocas listadas.
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5:** commit `feat: mês atual e ritmos seguem o ciclo do salário`.

### Task 5: tela Ciclo no backend (período e "veio do anterior")

**Files:**
- Modify: `backend/internal/usecase/planning/month_sheet.go` (campos), `backend/internal/usecase/insights/insights.go` (`MonthSheet`), `backend/internal/infra/http/month_api.go`, `summary_api.go`, `budget_api.go` (JSON)
- Test: `planning/month_sheet_test.go`, `http/month_api_test.go`

**Interfaces:**
- Produces:
  - `MonthSheet.StartsOn, EndsOn time.Time`; `MonthSheet.Carryover *float64` (sobra do ciclo anterior; `nil` quando o anterior não tem lançamento); `MonthSheet.LeftoverWithCarryover float64` (= `LeftoverWithPending` + carryover).
  - JSON de `/api/month`: `startsOn`, `endsOn` (`AAAA-MM-DD`), `carryover` (`null` ou número), `leftoverWithCarryover`. `/api/summary` e `/api/budget` ganham `startsOn`/`endsOn` do mês pedido.
- `Insights.MonthSheet` monta o anterior (`month.AddDate(0,-1,0)`) com o mesmo `BuildMonthSheet` e usa o `Leftover` dele.

- [ ] **Step 1: testes que falham:** `TestMonthSheetCarryover` (anterior com sobra 731,27 → `Carryover` = 731,27 e `LeftoverWithCarryover` soma); `TestMonthSheetNoPrevious` (anterior sem linhas → `Carryover == nil`); `TestMonthAPIPeriod` (resposta traz `startsOn` "2026-11-13" e `endsOn` "2026-12-14").
- [ ] **Step 2:** `go test ./internal/...` → FAIL.
- [ ] **Step 3:** implementar.
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5:** commit `feat: tela do ciclo com período e sobra do ciclo anterior`.

### Task 6: frontend

**Files (modify):** `frontend/src/features/month/MonthSheet.tsx`, `features/month/api.ts`, `features/month/lib/*`, `frontend/src/shared/components/MonthFilter.tsx`, o item "Mês" do menu lateral (em `App.tsx` ou no layout), textos de "mês" em `features/overview` e `features/budget` que se referem ao período.

**Interfaces:**
- Consumes: JSON da Task 5.
- Produces: `formatCycle(startsOn: string, endsOn: string): string` em `features/month/lib/` → `"13/nov – 14/dez"`.

- [ ] **Step 1: teste que falha** (vitest, ao lado dos `*.test.ts` existentes): `formatCycle("2026-11-13", "2026-12-14")` === `"13/nov – 14/dez"`.
- [ ] **Step 2:** `cd frontend && npm test -- formatCycle` → FAIL.
- [ ] **Step 3:** implementar. Título "Ciclo de novembro de 2026", com o período abaixo. Linha "Veio do ciclo anterior" no card Sobra quando `carryover != null`. O aviso de "em andamento" diz "Ciclo em andamento…". O menu passa de "Mês" para "Ciclo". O seletor continua `AAAA-MM` e mostra "ciclo de novembro de 2026".
- [ ] **Step 4:** `npm test`, `npm run build` e lint (`npx oxlint`) → PASS. Abrir a tela com o preview (`.claude/launch.json`) e conferir o título, o período e a linha de carryover.
- [ ] **Step 5:** commit `feat: tela Ciclo no lugar de Mês`.

### Task 7: documentação

**Files:** `docs/regras-de-calculo.md` (seções "Mês das compras no cartão", "Tela do mês" → "Tela do ciclo", "Mês em andamento", "Investimentos"), novo `docs/decisoes/0009-ciclo-do-salario.md` + linha em `docs/decisoes/README.md`, `CHANGELOG.md`.

- [ ] **Step 1:** escrever a regra do ciclo com as mesmas definições da seção Global Constraints e o ADR curto (contexto, decisão, consequências: histórico recalculado ao mudar o salário; gastos do dia do salário adiantado mudam de ciclo após a sincronização).
- [ ] **Step 2:** commit `docs: regra do ciclo do salário`.
