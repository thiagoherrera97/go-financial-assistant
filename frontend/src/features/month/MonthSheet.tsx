import './styles.css'
import type { ReactNode } from 'react'
import { CreditCard } from 'lucide-react'
import { MonthFilter } from '../../shared/components/MonthFilter'
import { QueryState } from '../../shared/components/QueryState'
import { StatTile } from '../../shared/components/StatTile'
import { formatBRL, formatMonthTitle, shortDay } from '../../shared/lib/format'
import { useMonth } from '../../shared/lib/useMonth'
import { cardNotes, leftoverLabel } from './lib/month'
import { useMonthSheet } from './api'
import type { SheetItem } from './api'

function Rows({ items, empty }: { items: SheetItem[]; empty: string }) {
  if (items.length === 0) return <p className="sheet-empty">{empty}</p>
  return (
    <ul className="sheet-rows">
      {items.map((it) => (
        <li key={it.label}>
          <span className="sheet-label">
            {it.label}
            {it.count > 1 && <span className="sheet-meta"> · {it.count}×</span>}
            {it.pending && <span className="sheet-meta"> · pendente</span>}
          </span>
          <span className="sheet-amount">{formatBRL(it.amount)}</span>
        </li>
      ))}
    </ul>
  )
}

function Block({ title, total, children }: { title: string; total: number; children: ReactNode }) {
  return (
    <section className="card sheet-block">
      <div className="sheet-head">
        <h2>{title}</h2>
        <strong>{formatBRL(total)}</strong>
      </div>
      {children}
    </section>
  )
}

/** O mês como na planilha: entradas de um lado, despesas (fixas, outras e a fatura de cada cartão por tipo) do outro. */
export default function MonthSheetPage() {
  const { month, setMonth, now } = useMonth()
  const sheet = useMonthSheet(month)

  return (
    <>
      <h1 className="page-title">{formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} warnPartial />

      <QueryState query={sheet}>
        {(s) => (
          <div className={sheet.isPlaceholderData ? 'is-stale' : undefined}>
            <div className="tiles">
              <StatTile label="Entradas" value={formatBRL(s.incomeTotal)} note={s.pendingTotal > 0 ? `+ ${formatBRL(s.pendingTotal)} a receber` : undefined} />
              <StatTile label="Despesas" value={formatBRL(s.expensesTotal)} note={`${formatBRL(s.cardsTotal)} no cartão`} />
              <StatTile
                label={leftoverLabel(s.pendingTotal > 0 ? s.leftoverWithPending : s.leftover)}
                value={formatBRL(Math.abs(s.pendingTotal > 0 ? s.leftoverWithPending : s.leftover))}
                note={s.pendingTotal > 0 ? `contando o salário a receber · hoje: ${formatBRL(s.leftover)}` : 'entradas − despesas'}
              />
            </div>

            <div className="sheet-grid section-gap">
              <div className="sheet-col">
                <Block title="Entradas" total={s.incomeTotal}>
                  <Rows items={s.income} empty="Nenhuma entrada no mês." />
                  {s.pending.length > 0 && (
                    <>
                      <h3 className="sheet-sub">A receber</h3>
                      <ul className="sheet-rows">
                        {s.pending.map((p) => (
                          <li key={p.label} className="is-pending">
                            <span className="sheet-label">
                              {p.label}
                              <span className="sheet-meta"> · cai {shortDay(p.date)}</span>
                            </span>
                            <span className="sheet-amount">{formatBRL(p.amount)}</span>
                          </li>
                        ))}
                      </ul>
                    </>
                  )}
                </Block>
                {s.invested !== 0 && (
                  <p className="sheet-aside">
                    {s.invested > 0 ? 'Aplicado' : 'Resgatado'} no mês (fora da sobra): <strong>{formatBRL(Math.abs(s.invested))}</strong>
                  </p>
                )}
              </div>

              <div className="sheet-col">
                <Block title="Despesas fixas" total={s.fixedTotal}>
                  <Rows items={s.fixed} empty="Nenhuma conta fixa fora do cartão." />
                </Block>
                {s.cards.map((c) => (
                  <Block key={c.name} title={`Fatura ${c.name}`} total={c.total}>
                    <Rows items={c.categories} empty="Nenhuma compra no cartão." />
                    {cardNotes(c).length > 0 && (
                      <ul className="sheet-rows sheet-notes">
                        {cardNotes(c).map((n) => (
                          <li key={n.label} title={n.hint}>
                            <span className="sheet-label">{n.label}</span>
                            <span className="sheet-amount">{formatBRL(n.amount)}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                    {c.refunds !== 0 && (
                      <p className="sheet-aside">
                        <CreditCard size={13} aria-hidden="true" /> Estornos já descontados: {formatBRL(c.refunds)}
                      </p>
                    )}
                  </Block>
                ))}
                <Block title="Outras despesas" total={s.otherTotal}>
                  <Rows items={s.other} empty="Nada além das fixas e do cartão." />
                </Block>
              </div>
            </div>
          </div>
        )}
      </QueryState>
    </>
  )
}
