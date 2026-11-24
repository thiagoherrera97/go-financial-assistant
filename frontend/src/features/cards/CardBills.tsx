import './styles.css'
import { CircleCheck, CircleAlert, Clock } from 'lucide-react'
import { useSearchParams } from 'react-router'
import { CategoryChip } from '../../shared/components/CategoryChip'
import { QueryState } from '../../shared/components/QueryState'
import { StatTile } from '../../shared/components/StatTile'
import { cleanLabel, installmentOf } from '../../shared/lib/labels'
import { formatBRL, shortDay } from '../../shared/lib/format'
import { useBillLines, useCardBills, type CardBill, type CardWithBills } from './api'
import { billCheck, billMonth, dayMonth, ORIGIN_LABEL, pickBill, STATE_LABEL } from './lib/bills'

/** A fatura de cada cartão, só ela: as compras que entram, o total e a conferência com o que o banco cobrou. */
export default function CardBills() {
  const cards = useCardBills()
  const [params, setParams] = useSearchParams()
  const set = (key: string, value: string | null) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === null) next.delete(key)
        else next.set(key, value)
        if (key === 'cartao') next.delete('fatura')
        return next
      },
      { replace: true },
    )

  return (
    <>
      <h1 className="page-title">Faturas do cartão</h1>
      <p className="page-subtitle">Só as compras de cada fatura, com o total conferido contra o que o banco cobrou.</p>
      <QueryState query={cards}>
        {({ cards: list }) => {
          if (list.length === 0) return <p className="state">Nenhum cartão de crédito conectado.</p>
          const card = list.find((c) => c.id === params.get('cartao')) ?? list[0]
          return (
            <>
              {list.length > 1 && (
                <div className="filters" role="group" aria-label="Cartão">
                  {list.map((c) => (
                    <button key={c.id} type="button" className="btn" aria-pressed={c.id === card.id} onClick={() => set('cartao', c.id)}>
                      {c.name}
                      {c.last4 && ` · ${c.last4}`}
                    </button>
                  ))}
                </div>
              )}
              <CardView key={card.id} card={card} wanted={params.get('fatura')} onPick={(due) => set('fatura', due)} />
            </>
          )
        }}
      </QueryState>
    </>
  )
}

function CardView({ card, wanted, onPick }: { card: CardWithBills; wanted: string | null; onPick: (due: string) => void }) {
  const i = pickBill(card.bills, wanted)
  if (i < 0) return <p className="state">Nenhuma compra com fatura neste cartão ainda.</p>
  const bill = card.bills[i]
  const prev = card.bills[i - 1]
  const next = card.bills[i + 1]
  return (
    <>
      <div className="filters cb-nav" role="group" aria-label="Fatura">
        <button type="button" className="btn" aria-label="Fatura anterior" disabled={!prev} onClick={() => prev && onPick(prev.due)}>
          ←
        </button>
        <select aria-label="Fatura" value={bill.due} onChange={(e) => onPick(e.target.value)}>
          {card.bills.map((b) => (
            <option key={b.due} value={b.due}>
              {billMonth(b.due)} · vence {dayMonth(b.due)} · {STATE_LABEL[b.state]}
            </option>
          ))}
        </select>
        <button type="button" className="btn" aria-label="Próxima fatura" disabled={!next} onClick={() => next && onPick(next.due)}>
          →
        </button>
      </div>
      <BillSummary bill={bill} card={card} first={i === 0} />
      <BillLines cardId={card.id} bill={bill} />
    </>
  )
}

function BillSummary({ bill, card, first }: { bill: CardBill; card: CardWithBills; first: boolean }) {
  const check = billCheck(bill, first)
  const Icon = check.tone === 'good' ? CircleCheck : check.tone === 'bad' ? CircleAlert : Clock
  const closed = bill.bankTotal !== null
  return (
    <>
      <div className="tiles">
        <StatTile
          label={closed ? 'Cobrado pelo banco' : 'Total até agora'}
          value={formatBRL(closed ? bill.bankTotal! : bill.total)}
          note={`vence ${shortDay(bill.due)}`}
        />
        <StatTile
          label="Lançamentos no app"
          value={formatBRL(bill.known)}
          note={bill.forecast !== 0 ? `${bill.count} lançamentos · ${formatBRL(bill.forecast)} em parcelas previstas` : `${bill.count} lançamentos`}
        />
        {closed ? (
          <StatTile
            label="Diferença"
            value={bill.difference === 0 ? 'Nenhuma' : formatBRL(bill.difference)}
            tone={bill.difference === 0 ? 'good' : first ? undefined : 'bad'}
            note={bill.difference === 0 ? 'centavos de arredondamento não contam' : 'banco menos lançamentos do app'}
          />
        ) : (
          <StatTile label="Saldo devedor no banco" value={formatBRL(card.bankBalance)} note="agora, somando as parcelas futuras" />
        )}
      </div>
      <p className={`cb-check is-${check.tone}`} role="note">
        <Icon size={18} aria-hidden="true" />
        <span>
          <strong>{check.title}</strong> {check.detail}
        </span>
      </p>
    </>
  )
}

function BillLines({ cardId, bill }: { cardId: string; bill: CardBill }) {
  const lines = useBillLines(cardId, bill.due)
  return (
    <section className="bill-section" aria-labelledby="cb-lines-title">
      <h2 className="bill-head" id="cb-lines-title">
        Compras da fatura de {billMonth(bill.due)} <span className="count">{bill.count}</span>
      </h2>
      <QueryState query={lines}>
        {({ lines: items }) =>
          items.length === 0 ? (
            <p className="state">Nenhuma compra nesta fatura.</p>
          ) : (
            <ul className="cb-list">
              {items.map((l) => {
                const tag = ORIGIN_LABEL[l.origin]
                const parcel = installmentOf(l.description)
                return (
                  <li key={l.id} className={`cb-row is-${l.origin.toLowerCase()}`}>
                    <CategoryChip category={l.category} size={16} />
                    <span className="cb-name">
                      <strong>{l.note || cleanLabel(l.description) || '—'}</strong>
                      <span className="bill-meta">
                        {l.origin !== 'GAP' && shortDay(l.purchaseDate ?? l.date)}
                        {parcel && ` · parcela ${parcel}`}
                        {l.origin !== 'GAP' && ` · ${l.categoryLabel}`}
                        {l.pending && ' · pendente'}
                        {l.note && ` · no extrato: ${cleanLabel(l.description) || '—'}`}
                      </span>
                    </span>
                    {tag && <span className="cb-tag">{tag}</span>}
                    <span className={`bill-amount ${l.amount < 0 ? 'is-refund' : ''}`}>{formatBRL(l.amount)}</span>
                  </li>
                )
              })}
            </ul>
          )
        }
      </QueryState>
    </section>
  )
}
