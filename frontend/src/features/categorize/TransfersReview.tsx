import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Undo2 } from 'lucide-react'
import { useState } from 'react'
import { ApiError } from '../../shared/api/request'
import { Pager } from '../../shared/components/Pager'
import { QueryState } from '../../shared/components/QueryState'
import { formatBRL } from '../../shared/lib/format'
import { cleanLabel } from '../../shared/lib/labels'
import { decideTransfer, undoTransfer, useTransfersReview } from './api'
import type { TransferDecision, TransferItem } from './api'
import { DECISION_LABEL, canBePayment, decisionNote, totals, transferSummary, truncatedNote } from './lib/transfers'
import { pageOf } from './lib/categorize'

const PAGE_SIZE = 10

/** Transferências que o Pluggy trouxe (Pix/TED): só você sabe se foi para outra conta sua, para uma pessoa ou um pagamento. */
export default function TransfersReview() {
  const query = useTransfersReview()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  // Categoria escolhida para "Pagamento", por transferência.
  const [category, setCategory] = useState<Record<string, string>>({})

  // Totais, gráficos e Projeção mudam com a decisão (e voltam ao desfazer): atualiza tudo.
  const refresh = () => queryClient.invalidateQueries()
  const decide = useMutation({
    mutationFn: (v: { id: string; decision: TransferDecision; category?: string }) => decideTransfer(v.id, v.decision, v.category),
    onSuccess: refresh,
  })
  const undo = useMutation({ mutationFn: (id: string) => undoTransfer(id), onSuccess: refresh })
  const busy = decide.isPending || undo.isPending
  const error = decide.error ?? undo.error

  return (
    <section className="section-gap" aria-labelledby="transfers-title">
      <h2 className="chart-title" id="transfers-title">
        Transferências a revisar
      </h2>
      <QueryState query={query}>
        {(data) => {
          const { items, page: current } = pageOf(data.pending, page, PAGE_SIZE)
          const label = (value: string) => data.categories.find((c) => c.value === value)?.label ?? value
          const sums = totals(data.pending)
          return (
            <>
              <p className="tile-note">
                Pix e TED que o banco chamou de transferência e que hoje contam como gasto ou renda. Diga o que cada uma é: &quot;Entre
                contas minhas&quot; deixa de contar (e pode ser desfeita); &quot;Para uma pessoa&quot; vai para Pessoas; &quot;Pagamento&quot; vai
                para a categoria que você escolher. Nada muda sem a sua escolha e nenhum nome vai à IA.
              </p>
              {data.pending.length > 0 && (
                <p className="tile-note">
                  {data.pendingTotal} {data.pendingTotal === 1 ? 'pendente' : 'pendentes'} na lista: {formatBRL(sums.out)} em saídas e{' '}
                  {formatBRL(sums.in)} em entradas. {truncatedNote(data.pending.length, data.pendingTotal)}
                </p>
              )}
              {error && (
                <p className="state error" role="alert">
                  {error instanceof ApiError ? error.message : 'Não foi possível salvar. Tente de novo.'}
                </p>
              )}

              {data.pending.length === 0 ? (
                <p className="state">Nenhuma transferência a revisar.</p>
              ) : (
                <ul className="classify-list">
                  {items.map((t) => (
                    <PendingRow
                      key={t.id}
                      item={t}
                      categories={data.categories}
                      category={category[t.id] ?? ''}
                      onCategory={(value) => setCategory((prev) => ({ ...prev, [t.id]: value }))}
                      busy={busy}
                      onDecide={(decision, cat) => decide.mutate({ id: t.id, decision, category: cat })}
                    />
                  ))}
                </ul>
              )}
              {data.pending.length > PAGE_SIZE && (
                <Pager page={current} limit={PAGE_SIZE} total={data.pending.length} onPage={setPage} noun={['transferência', 'transferências']} />
              )}

              {data.decided.length > 0 && (
                <>
                  <h3 className="chart-title section-gap">Decididas</h3>
                  <ul className="classify-list">
                    {data.decided.map((t) => (
                      <li key={t.id} className="classify-row transfer-row">
                        <span className="classify-name">
                          <strong>{cleanLabel(t.description)}</strong>
                          <span className="badge">{t.decision ? DECISION_LABEL[t.decision] : ''}</span>
                          <span className="bill-meta">
                            {transferSummary(t)} · {decisionNote(t, label)}
                          </span>
                        </span>
                        <button type="button" className="btn btn-small" disabled={busy} onClick={() => undo.mutate(t.id)}>
                          <Undo2 size={13} aria-hidden="true" /> Desfazer
                        </button>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </>
          )
        }}
      </QueryState>
    </section>
  )
}

function PendingRow({
  item,
  categories,
  category,
  onCategory,
  busy,
  onDecide,
}: {
  item: TransferItem
  categories: { value: string; label: string }[]
  category: string
  onCategory: (value: string) => void
  busy: boolean
  onDecide: (decision: TransferDecision, category?: string) => void
}) {
  const name = cleanLabel(item.description)
  return (
    <li className="classify-row transfer-row">
      <span className="classify-name">
        <strong>{name}</strong>
        <span className="bill-meta">{transferSummary(item)}</span>
      </span>
      <span className="transfer-actions">
        <button type="button" className="btn btn-small" disabled={busy} onClick={() => onDecide('OWN')}>
          {DECISION_LABEL.OWN}
        </button>
        <button type="button" className="btn btn-small" disabled={busy} onClick={() => onDecide('PERSON')}>
          {DECISION_LABEL.PERSON}
        </button>
        {canBePayment(item) && (
          <span className="transfer-payment">
            <label>
              <span className="sr-only">Categoria do pagamento de {name}</span>
              <select value={category} onChange={(e) => onCategory(e.target.value)}>
                <option value="">Categoria…</option>
                {categories.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </select>
            </label>
            <button type="button" className="btn btn-small" disabled={busy || !category} onClick={() => onDecide('PAYMENT', category)}>
              {DECISION_LABEL.PAYMENT}
            </button>
          </span>
        )}
      </span>
    </li>
  )
}
