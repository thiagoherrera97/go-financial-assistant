import './styles.css'
import type { ReactNode } from 'react'
import { CreditCard } from 'lucide-react'
import { MonthFilter } from '../../shared/components/MonthFilter'
import { QueryState } from '../../shared/components/QueryState'
import { StatTile } from '../../shared/components/StatTile'
import { formatBRL, formatMonthTitle, shortDay } from '../../shared/lib/format'
import { useMonth } from '../../shared/lib/useMonth'
import { cardNotes, formatCycle, leftoverLabel, sheetLeftover } from './lib/month'
import { useMonthSheet } from './api'
import type { SheetItem } from './api'

function Head({ it }: { it: SheetItem }) {
  return (
    <>
      <span className="sheet-label">
        {it.label}
        {it.count > 1 && <span className="sheet-meta"> · {it.count}×</span>}
        {it.pending && <span className="sheet-meta"> · pendente</span>}
      </span>
      <span className="sheet-amount">{formatBRL(it.amount)}</span>
    </>
  )
}

/** Linhas da planilha. Com mais de um lançamento (ou nota), a linha abre e mostra cada um, com a nota. */
function Rows({ items, empty }: { items: SheetItem[]; empty: string }) {
  if (items.length === 0) return <p className="sheet-empty">{empty}</p>
  return (
    <ul className="sheet-rows">
      {items.map((it) => {
        const single = it.lines.length === 1 ? it.lines[0] : null
        if (single || it.lines.length === 0) {
          return (
            <li key={it.label}>
              <div className="sheet-line">
                <Head it={it} />
              </div>
              {single?.note && <p className="sheet-note">{single.note}</p>}
            </li>
          )
        }
        const notes = it.lines.filter((l) => l.note).length
        return (
          <li key={it.label}>
            <details>
              <summary className="sheet-line">
                <Head it={it} />
              </summary>
              {notes > 0 && <span className="sheet-meta">{notes === 1 ? '1 nota' : `${notes} notas`}</span>}
              <ul className="sheet-detail">
                {it.lines.map((l, i) => (
                  <li key={`${l.date}-${l.label}-${i}`}>
                    <div className="sheet-line">
                      <span className="sheet-label">
                        <span className="sheet-meta">{l.date.slice(8, 10)}/{l.date.slice(5, 7)}</span> {l.label}
                      </span>
                      <span className="sheet-amount">{formatBRL(l.amount)}</span>
                    </div>
                    {l.note && <p className="sheet-note">{l.note}</p>}
                  </li>
                ))}
              </ul>
            </details>
          </li>
        )
      })}
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

/** O ciclo do salário como na planilha: entradas de um lado, despesas (fixas, outras e a fatura de cada cartão por tipo) do outro. */
export default function MonthSheetPage() {
  const { month, setMonth, now } = useMonth()
  const sheet = useMonthSheet(month)

  return (
    <>
      <h1 className="page-title">Ciclo de {formatMonthTitle(month).toLowerCase()}</h1>
      {sheet.data?.startsOn && <p className="page-subtitle">{formatCycle(sheet.data.startsOn, sheet.data.endsOn)}</p>}
      <MonthFilter month={month} now={now} onChange={setMonth} warnPartial />

      <QueryState query={sheet}>
        {(s) => (
          <div className={sheet.isPlaceholderData ? 'is-stale' : undefined}>
            <div className="tiles">
              <StatTile label="Entradas" value={formatBRL(s.incomeTotal + s.pendingTotal)} note={s.pendingTotal > 0 ? `${formatBRL(s.pendingTotal)} ainda a receber` : undefined} />
              <StatTile label="Despesas" value={formatBRL(s.expensesTotal)} note={`${formatBRL(s.cardsTotal)} no cartão`} />
              <StatTile label={leftoverLabel(sheetLeftover(s).value)} value={formatBRL(Math.abs(sheetLeftover(s).value))} note={sheetLeftover(s).note} />
            </div>

            <div className="sheet-grid section-gap">
              <div className="sheet-col">
                <Block title="Entradas" total={s.incomeTotal + s.pendingTotal}>
                  <Rows items={s.income} empty="Nenhuma entrada no ciclo." />
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
                    {s.invested > 0 ? 'Aplicado' : 'Resgatado'} no ciclo (fora da sobra): <strong>{formatBRL(Math.abs(s.invested))}</strong>
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
