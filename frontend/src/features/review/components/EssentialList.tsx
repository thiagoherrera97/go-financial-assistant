import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router'
import { CategoryChip } from '../../../shared/components/CategoryChip'
import { NoteEditor } from '../../../shared/components/NoteEditor'
import { QueryState } from '../../../shared/components/QueryState'
import { setClassification, useSubcategorySpend } from '../../../shared/api/subcategories'
import { useExpectedExpenses } from '../../../shared/api/expectedIncome'
import { cleanLabel, installmentOf } from '../../../shared/lib/labels'
import { formatBRL, shortDay } from '../../../shared/lib/format'
import { useExpenseList, type ExpenseItem } from '../api'
import { ESSENTIAL_TABS, essentialQuery, type EssentialTab } from '../lib/essential'

const PAGE = 20

/**
 * As despesas do mês separadas em supérfluas, essenciais e a revisar, do maior valor para o menor, com o botão para mudar
 * cada uma ali mesmo (vale só para aquela despesa; para o estabelecimento inteiro, use "categoria" nas Transações).
 */
export function EssentialList({ month }: { month: string }) {
  const [tab, setTab] = useState<EssentialTab>('false')
  const [limit, setLimit] = useState(PAGE)
  const spend = useSubcategorySpend(month)
  const txs = useExpenseList(essentialQuery(month, tab, limit))
  // Contas fixas do ciclo que ainda não saíram (aluguel, financiamento, luz...): ainda não são transação, mas já são gasto
  // essencial do mês. Entram na aba Essenciais e no total dela, marcadas como previstas.
  const toPay = useExpectedExpenses(month, true)
  const expected = toPay.data?.items ?? []
  const expectedTotal = toPay.data?.total ?? 0
  const queryClient = useQueryClient()

  const change = useMutation({
    mutationFn: ({ tx, essential }: { tx: ExpenseItem; essential: boolean }) =>
      setClassification(tx.id, { category: tx.category, subcategory: tx.subcategory ?? '', essential, applyToAll: false }),
    onSuccess: () => {
      for (const key of ['transactions', 'transaction-groups', 'subcategory-spend', 'breakdown', 'month-sheet', 'summary', 'categorize']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    },
  })

  const totals = spend.data ? { false: spend.data.superfluous, true: spend.data.essential + expectedTotal, unknown: spend.data.unknown } : null
  const linkAll = `/transacoes?${new URLSearchParams({ mes: month, tipo: 'EXPENSE', essencial: tab })}`

  return (
    <section aria-labelledby="essential-title">
      <h2 className="bill-head" id="essential-title">
        Supérfluos e essenciais do mês
      </h2>
      <div className="filters" role="group" aria-label="Tipo de gasto">
        {ESSENTIAL_TABS.map((t) => (
          <button
            key={t.value}
            type="button"
            className="btn"
            aria-pressed={tab === t.value}
            onClick={() => {
              setTab(t.value)
              setLimit(PAGE)
            }}
          >
            {t.label}
            {totals && ` · ${formatBRL(totals[t.value])}`}
          </button>
        ))}
      </div>

      {change.isError && (
        <p className="state error" role="alert">
          Não foi possível mudar. Tente de novo.
        </p>
      )}
      {tab === 'true' && expected.length > 0 && (
        <>
          <h3 className="chart-title">Contas fixas que ainda vão sair</h3>
          <ul className="ess-list ess-expected">
            {expected.map((e) => (
              <li key={`${e.label}:${e.date}`} className="ess-row">
                <CategoryChip category="BILLS" size={18} />
                <span className="ess-name">
                  <strong>{e.label}</strong>
                  <span className="bill-meta">
                    {shortDay(e.date)} · {e.late ? 'atrasada: ainda não saiu da conta' : 'prevista'}
                  </span>
                </span>
                <span className="bill-amount">{formatBRL(e.amount)}</span>
              </li>
            ))}
          </ul>
          <h3 className="chart-title section-gap">Já lançadas</h3>
        </>
      )}
      <QueryState query={txs}>
        {(page) =>
          page.items.length === 0 ? (
            <p className="state">Nenhuma despesa {ESSENTIAL_TABS.find((t) => t.value === tab)!.empty} neste mês.</p>
          ) : (
            <>
              <ul className="ess-list">
                {page.items.map((tx) => (
                  <li key={tx.id} className="ess-row">
                    <CategoryChip category={tx.category} size={18} />
                    <span className="ess-name">
                      <strong className={tx.note ? 'tx-title-note' : undefined}>{tx.note || cleanLabel(tx.description) || '—'}</strong>
                      <span className="bill-meta">
                        {shortDay(tx.purchaseDate ?? tx.date)}
                        {installmentOf(tx.description) && ` · parcela ${installmentOf(tx.description)}`} · {tx.subcategoryLabel || tx.categoryLabel}
                        {tx.note && ` · no extrato: ${cleanLabel(tx.description) || '—'}`}
                      </span>
                    </span>
                    <span className="bill-amount">{formatBRL(tx.amount)}</span>
                    <span className="ess-actions">
                      <NoteEditor key={`${tx.id}:${tx.note}`} id={tx.id} note={tx.note} />
                      {tx.essential !== false && (
                        <button type="button" className="btn btn-small" disabled={change.isPending} onClick={() => change.mutate({ tx, essential: false })}>
                          É supérfluo
                        </button>
                      )}
                      {tx.essential !== true && (
                        <button type="button" className="btn btn-small" disabled={change.isPending} onClick={() => change.mutate({ tx, essential: true })}>
                          É essencial
                        </button>
                      )}
                    </span>
                  </li>
                ))}
              </ul>
              <p className="ess-more">
                {page.total > page.items.length && limit < 100 ? (
                  <button type="button" className="btn btn-small" onClick={() => setLimit(Math.min(limit + PAGE, 100))}>
                    Ver mais ({page.total - page.items.length})
                  </button>
                ) : page.total > page.items.length ? (
                  <Link className="link" to={linkAll}>
                    Ver todas as {page.total} nas transações
                  </Link>
                ) : null}
              </p>
            </>
          )
        }
      </QueryState>
      <p className="tile-note">A mudança vale só para aquela despesa. Para corrigir todas de um estabelecimento, use “categoria” nas Transações.</p>
    </section>
  )
}
