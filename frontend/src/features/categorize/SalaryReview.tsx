import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Undo2 } from 'lucide-react'
import { useState } from 'react'
import { ApiError } from '../../shared/api/request'
import { QueryState } from '../../shared/components/QueryState'
import { formatBRL, shortDay } from '../../shared/lib/format'
import { cleanLabel } from '../../shared/lib/labels'
import { decideSalary, undoSalary, useSalaryReview } from './api'
import type { SalaryDecision, SalaryReviewItem } from './api'
import { decidedNote, decisionOf, initialChoice, salaryOptions } from './lib/salary'

/**
 * Créditos de salário que o app não reconheceu com certeza: o salário cheio que veio de uma vez, uma parte com valor
 * diferente do combinado, ou um extra (hora extra, PLR, 13º). Só aparece com o salário configurado em Configurações → Renda.
 */
export default function SalaryReview() {
  const query = useSalaryReview()
  const queryClient = useQueryClient()
  // A resposta escolhida em cada crédito, antes de confirmar.
  const [choice, setChoice] = useState<Record<string, string>>({})
  // O "a receber", os totais e o início do mês mudam com a decisão: atualiza tudo.
  const refresh = () => queryClient.invalidateQueries()
  const decide = useMutation({ mutationFn: (v: { id: string; decision: SalaryDecision }) => decideSalary(v.id, v.decision), onSuccess: refresh })
  const undo = useMutation({ mutationFn: (id: string) => undoSalary(id), onSuccess: refresh })
  const busy = decide.isPending || undo.isPending
  const error = decide.error ?? undo.error

  if (query.data && query.data.parts.length === 0 && query.data.pending.length === 0 && query.data.decided.length === 0) return null
  return (
    <section className="section-gap" aria-labelledby="salary-review-title">
      <h2 className="chart-title" id="salary-review-title">
        Conferir salário
      </h2>
      <QueryState query={query}>
        {(data) => (
          <>
            <p className="tile-note">
              Créditos com cara de salário que o app não reconheceu com certeza: o salário que veio todo de uma vez, uma parte com
              valor diferente do combinado, ou um extra. Diga o que cada um é: o &quot;a receber&quot;, os totais e o início do mês passam
              a seguir a sua resposta, e dá para desfazer.
            </p>
            {error && (
              <p className="state error" role="alert">
                {error instanceof ApiError ? error.message : 'Não foi possível salvar. Tente de novo.'}
              </p>
            )}
            {data.pending.length === 0 ? (
              <p className="state">Nada a conferir.</p>
            ) : (
              <ul className="classify-list">
                {data.pending.map((item) => (
                  <PendingRow
                    key={item.id}
                    item={item}
                    parts={data.parts}
                    value={choice[item.id] ?? initialChoice(item)}
                    onChange={(value) => setChoice((prev) => ({ ...prev, [item.id]: value }))}
                    busy={busy}
                    onConfirm={(value) => decide.mutate({ id: item.id, decision: decisionOf(value, item.month) })}
                  />
                ))}
              </ul>
            )}
            {data.decided.length > 0 && (
              <>
                <h3 className="chart-title section-gap">Conferidos</h3>
                <ul className="classify-list">
                  {data.decided.map((d) => (
                    <li key={d.id} className="classify-row transfer-row">
                      <span className="classify-name">
                        <strong>{cleanLabel(d.label) || 'Salário'}</strong>
                        <span className="badge">{decidedNote(d)}</span>
                        <span className="bill-meta">
                          {shortDay(d.date)} · {formatBRL(d.amount)}
                        </span>
                      </span>
                      <button type="button" className="btn btn-small" disabled={busy} onClick={() => undo.mutate(d.id)}>
                        <Undo2 size={13} aria-hidden="true" /> Desfazer
                      </button>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </>
        )}
      </QueryState>
    </section>
  )
}

function PendingRow({
  item,
  parts,
  value,
  onChange,
  busy,
  onConfirm,
}: {
  item: SalaryReviewItem
  parts: { day: number; amount: number }[]
  value: string
  onChange: (value: string) => void
  busy: boolean
  onConfirm: (value: string) => void
}) {
  const name = cleanLabel(item.label) || 'Salário'
  const guess = item.suggest === '' ? 'parece um extra' : `o app supôs: ${salaryOptions(item, parts).find((o) => o.value === initialChoice(item))?.label}`
  return (
    <li className="classify-row transfer-row">
      <span className="classify-name">
        <strong>{name}</strong>
        <span className="bill-meta">
          {shortDay(item.date)} · {formatBRL(item.amount)} · {guess}
        </span>
      </span>
      <span className="transfer-actions">
        <label>
          <span className="sr-only">O que é o crédito de {formatBRL(item.amount)} de {shortDay(item.date)}</span>
          <select value={value} onChange={(e) => onChange(e.target.value)}>
            <option value="">O que é?</option>
            {salaryOptions(item, parts).map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <button type="button" className="btn btn-small" disabled={busy || !value} onClick={() => onConfirm(value)}>
          Confirmar
        </button>
      </span>
    </li>
  )
}
