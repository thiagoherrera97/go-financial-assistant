import { CalendarDays, LayoutGrid, List } from 'lucide-react'
import { useEffect, useState } from 'react'
import { CategoryChip } from '../../shared/components/CategoryChip'
import { GroupCard } from './components/GroupCard'
import { QueryState } from '../../shared/components/QueryState'
import { StatTile } from '../../shared/components/StatTile'
import { TransactionTable } from './components/TransactionTable'
import { categoryVisual } from '../../shared/lib/categoryVisual'
import { formatBRL, formatMonthTitle } from '../../shared/lib/format'
import { CATEGORIES, KINDS, PAYMENT_METHODS } from '../../shared/lib/labels'
import { FORECAST_MONTHS_AHEAD, shiftMonth } from '../../shared/lib/months'
import { formatDayLabel, incomeSources, INCOME_GROUP, investSummary, withGroupFilter } from './lib/transactions'
import { useTransactionFilters } from './lib/useTransactionFilters'
import { useTransactionGroups, useTransactions } from './api'
import { useAccounts } from '../../shared/api/accounts'
import type { TransactionGroup } from './api'
import { Pager } from '../../shared/components/Pager'
import { useSubcategoryCatalog } from '../../shared/api/subcategories'

const ESSENTIAL_FILTERS = [
  { value: 'false', label: 'Supérfluo' },
  { value: 'true', label: 'Essencial' },
  { value: 'unknown', label: 'A revisar' },
]

export default function Transactions() {
  const { params, now, month, query, view, update } = useTransactionFilters()
  const accounts = useAccounts()
  // Os totais do topo vêm dos grupos por categoria, agregados no SQL, qualquer que seja a visão.
  const categories = useTransactionGroups(query, 'category')
  // Dá para olhar até FORECAST_MONTHS_AHEAD meses à frente: as parcelas previstas dos cartões vão até lá.
  const maxMonth = shiftMonth(now, FORECAST_MONTHS_AHEAD)

  const catalog = useSubcategoryCatalog()
  const subOptions = (catalog.data?.subcategories ?? [])
    .filter((s) => s.category === params.get('categoria'))
    .map((s) => ({ value: s.code, label: s.label }))

  const select = (key: string, label: string, options: { value: string; label: string }[], also: Record<string, string> = {}) => (
    <label className="field">
      <span className="sr-only">{label}</span>
      <select value={params.get(key) ?? ''} onChange={(e) => update({ [key]: e.target.value, ...also })}>
        <option value="">{label}</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  )

  return (
    <>
      <h1 className="page-title">Transações{month ? ` de ${formatMonthTitle(month)}` : ''}</h1>

      <div className="filters">
        <button type="button" className="btn" aria-label="Ciclo anterior" disabled={!month} onClick={() => month && update({ mes: shiftMonth(month, -1) })}>
          ←
        </button>
        <input
          type="month"
          aria-label="Mês"
          value={month ?? ''}
          max={maxMonth}
          disabled={!month}
          onChange={(e) => e.target.value && update({ mes: e.target.value })}
        />
        <button type="button" className="btn" aria-label="Próximo ciclo" disabled={!month || month >= maxMonth} onClick={() => month && update({ mes: shiftMonth(month, 1) })}>
          →
        </button>
        <label className="check">
          <input type="checkbox" checked={!month} onChange={(e) => update({ mes: e.target.checked ? 'todos' : now })} />
          Todos os meses
        </label>
      </div>
      <div className="filters">
        {select('tipo', 'Tipo', KINDS)}
        {select('categoria', 'Categoria', CATEGORIES, { sub: '' })}
        {subOptions.length > 0 && select('sub', 'Subcategoria', subOptions)}
        {select('essencial', 'Essencial ou supérfluo', ESSENTIAL_FILTERS)}
        {select('forma', 'Forma de pagamento', PAYMENT_METHODS)}
        {select('conta', 'Conta ou cartão', (accounts.data ?? []).map((a) => ({ value: a.id, label: `${a.name} ${a.last4}`.trim() })))}
        <SearchBox value={params.get('q') ?? ''} onCommit={(q) => update({ q })} />
      </div>

      <Summary groups={categories.data} />

      <div className="view-toggle" role="group" aria-label="Forma de exibir">
        {VIEWS.map(({ key, label, Icon }) => (
          <button key={key} type="button" className="btn" aria-pressed={view === key} onClick={() => update({ vista: key === 'categoria' ? '' : key })}>
            <Icon size={16} aria-hidden="true" /> {label}
          </button>
        ))}
      </div>

      {view === 'lista' && <ListView query={query} onPage={(n) => update({ pagina: n > 1 ? String(n) : '' })} />}
      {view === 'categoria' && (
        <QueryState query={categories}>
          {(groups) => <CategoryCards groups={groups} query={query} stale={categories.isPlaceholderData} />}
        </QueryState>
      )}
      {view === 'dia' && <DayCards query={query} />}
    </>
  )
}

const VIEWS = [
  { key: 'categoria', label: 'Por categoria', Icon: LayoutGrid },
  { key: 'dia', label: 'Por dia', Icon: CalendarDays },
  { key: 'lista', label: 'Lista', Icon: List },
] as const

/** Totais do que está filtrado: despesas e receitas. Movimento de caixinhas e investimentos fica de fora: não é dinheiro
 * a mais, só troca de lugar (o rendimento já entra em Receitas). */
function Summary({ groups }: { groups: TransactionGroup[] | undefined }) {
  if (!groups) return null
  const total = groups.reduce(
    (acc, g) => ({
      expense: acc.expense + g.expense,
      income: acc.income + g.income,
      count: acc.count + g.count,
    }),
    { expense: 0, income: 0, count: 0 },
  )
  return (
    <div className="tiles section-gap">
      <StatTile label="Despesas" value={formatBRL(total.expense)} />
      <StatTile label="Receitas" value={formatBRL(total.income)} />
      <StatTile label="Lançamentos" value={String(total.count)} />
    </div>
  )
}

function CategoryCards({ groups: all, query, stale }: { groups: TransactionGroup[]; query: string; stale: boolean }) {
  // Caixinha não aparece: só troca dinheiro de lugar (o rendimento já está em Entradas).
  const groups = all.filter((g) => g.key !== 'RESERVE')
  if (groups.length === 0) return <p className="state">Nenhuma transação encontrada.</p>
  const max = Math.max(...groups.map((g) => Math.max(g.expense, g.income, Math.abs(g.applied - g.redeemed))), 1)
  return (
    <div className={`group-grid chart-body ${stale ? 'is-stale' : ''}`}>
      {groups.map((g) => {
        const { color } = categoryVisual(g.key)
        return (
          <GroupCard
            key={g.key}
            color={color}
            icon={<CategoryChip category={g.key} size={20} />}
            title={g.label}
            subtitle={`${g.count} ${g.count === 1 ? 'lançamento' : 'lançamentos'}`}
            {...amounts(g)}
            share={Math.max(g.expense, g.income, Math.abs(g.applied - g.redeemed)) / max}
          >
            {() =>
              g.key === INCOME_GROUP ? (
                <IncomeDetails query={withGroupFilter(query, 'category', g.key)} />
              ) : (
                <GroupDetails query={withGroupFilter(query, 'category', g.key)} hideCategory />
              )
            }
          </GroupCard>
        )
      })}
    </div>
  )
}

function DayCards({ query }: { query: string }) {
  const days = useTransactionGroups(query, 'day')
  return (
    <QueryState query={days}>
      {(groups) => {
        if (groups.length === 0) return <p className="state">Nenhuma transação encontrada.</p>
        const max = Math.max(...groups.map((g) => Math.max(g.expense, g.income, Math.abs(g.applied - g.redeemed))), 1)
        return (
          <div className={`group-grid chart-body ${days.isPlaceholderData ? 'is-stale' : ''}`}>
            {groups.map((g) => (
              <GroupCard
                key={g.key}
                color="var(--accent)"
                icon={<span className="chip chip-day">{g.key.slice(8)}</span>}
                title={formatDayLabel(g.key)}
                subtitle={`${g.count} ${g.count === 1 ? 'lançamento' : 'lançamentos'}`}
                {...amounts(g)}
                share={Math.max(g.expense, g.income, Math.abs(g.applied - g.redeemed)) / max}
              >
                {() => <GroupDetails query={withGroupFilter(query, 'day', g.key)} />}
              </GroupCard>
            ))}
          </div>
        )
      }}
    </QueryState>
  )
}

/**
 * Valor do cabeçalho do card: a despesa quando há. Sem despesa, o grupo é de receita ou de
 * investimento, e o valor vem com uma legenda curta embaixo (nunca texto longo no lugar do valor).
 */
function amounts(g: TransactionGroup) {
  const invest = g.transfer > 0 ? investSummary(g.applied, g.redeemed, g.key === 'RESERVE') : null
  const extras = [g.income > 0 && `Receitas ${formatBRL(g.income)}`, invest && invest.headline].filter((e): e is string => !!e)
  if (g.expense > 0) return { amount: `-${formatBRL(g.expense)}`, extras }
  if (g.income > 0) return { amount: `+${formatBRL(g.income)}`, extras: ['Receitas', ...(invest ? [invest.headline] : [])] }
  if (invest) return { amount: invest.signed, extras: [invest.headline, invest.detail] }
  return { amount: formatBRL(0), extras: [] }
}

/** Lançamentos de um grupo aberto; busca só quando o card é expandido. */
function GroupDetails({ query, hideCategory = false }: { query: string; hideCategory?: boolean }) {
  const result = useTransactions(query)
  return (
    <QueryState query={result}>
      {(page) => (
        <>
          <TransactionTable items={page.items} hideCategory={hideCategory} />
          {page.total > page.items.length && (
            <p className="tile-note">Mostrando {page.items.length} de {page.total}. Refine os filtros para ver o restante.</p>
          )}
        </>
      )}
    </QueryState>
  )
}

/** Entradas abertas: as receitas separadas por fonte (salário, Pix de cada pessoa, rendimentos...). */
function IncomeDetails({ query }: { query: string }) {
  const result = useTransactions(query)
  return (
    <QueryState query={result}>
      {(page) => {
        const sources = incomeSources(page.items)
        const max = Math.max(...sources.map((s) => s.total), 1)
        return (
          <div className="income-sources">
            {sources.map((s) => (
              <GroupCard
                key={s.key}
                color="var(--cat-salary)"
                icon={<CategoryChip category={s.items[0].category === 'SALARY' ? 'SALARY' : INCOME_GROUP} size={16} />}
                title={s.label}
                subtitle={`${s.items.length} ${s.items.length === 1 ? 'entrada' : 'entradas'}`}
                amount={`+${formatBRL(s.total)}`}
                share={s.total / max}
              >
                {() => <TransactionTable items={s.items} />}
              </GroupCard>
            ))}
            {page.total > page.items.length && (
              <p className="tile-note">Somando as {page.items.length} primeiras de {page.total}. Refine os filtros para ver o restante.</p>
            )}
          </div>
        )
      }}
    </QueryState>
  )
}

function ListView({ query, onPage }: { query: string; onPage: (n: number) => void }) {
  const result = useTransactions(query)
  return (
    <QueryState query={result}>
      {(page) => (
        <section className={`card chart-body ${result.isPlaceholderData ? 'is-stale' : ''}`} aria-label="Lista de transações">
          {page.items.length === 0 ? <p className="state">Nenhuma transação encontrada.</p> : <TransactionTable items={page.items} />}
          <Pager page={page.page} limit={page.limit} total={page.total} onPage={onPage} />
        </section>
      )}
    </QueryState>
  )
}

/** Busca com espera: só consulta a API 300ms depois da última tecla. */
function SearchBox({ value, onCommit }: { value: string; onCommit: (q: string) => void }) {
  const [text, setText] = useState(value)
  // Se a URL mudar por fora (botão voltar), o campo acompanha.
  const [seen, setSeen] = useState(value)
  if (value !== seen) {
    setSeen(value)
    setText(value)
  }
  useEffect(() => {
    if (text === value) return
    const id = setTimeout(() => onCommit(text), 300)
    return () => clearTimeout(id)
  }, [text, value, onCommit])

  return (
    <label className="field">
      <span className="sr-only">Buscar na descrição</span>
      <input type="search" placeholder="Buscar na descrição" maxLength={100} value={text} onChange={(e) => setText(e.target.value)} />
    </label>
  )
}
