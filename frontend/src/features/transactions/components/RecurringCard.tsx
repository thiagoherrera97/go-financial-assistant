import './RecurringCard.css'
import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowDownLeft, ArrowUpRight, CircleCheck, Clock, Pencil, Plus, Trash2 } from 'lucide-react'
import { formatBRL, shortDay } from '../../../shared/lib/format'
import { createFlow, deleteFlow, updateFlow, useRecurringFlows, type FlowInput, type RecurringFlow } from '../api'
import { emptyFlowForm, flowRule, flowToForm, toFlowInput, type FlowForm } from '../lib/flows'

/** Entradas e saídas que se repetem todo mês fora o salário (aluguel, seguro, financiamento, Pix da família). Alimentam o
 * "a receber" e o "a pagar" de Transações e a previsão do cheque especial. */
export function RecurringCard() {
  const query = useRecurringFlows()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<RecurringFlow | 'in' | 'out' | null>(null)
  const [startMonth, setStartMonth] = useState('')
  const refresh = () =>
    Promise.all(['recurring-flows', 'debts', 'expected-income', 'expected-expenses'].map((key) => queryClient.invalidateQueries({ queryKey: [key] })))
  const save = useMutation({
    mutationFn: (input: FlowInput) => (editing && typeof editing === 'object' ? updateFlow(editing.id, input) : createFlow(input)),
    onSuccess: () => {
      setEditing(null)
      refresh()
    },
  })
  const remove = useMutation({ mutationFn: deleteFlow, onSuccess: refresh })
  const flows = query.data ?? []
  const open = (kind: 'in' | 'out') => {
    const now = new Date()
    setStartMonth(`${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`)
    save.reset()
    setEditing(kind)
  }
  const fmt = formatBRL

  const group = (isIn: boolean) => {
    const list = flows.filter((f) => f.in === isIn && !f.ended)
    const total = list.filter((f) => f.active).reduce((s, f) => s + f.thisMonth, 0)
    return (
      <div className="rec-flow-group">
        <h3>
          {isIn ? 'Entradas' : 'Saídas'} <span className="tile-note">este mês: {fmt(total)}</span>
        </h3>
        {list.length === 0 && <p className="tile-note">Nenhuma cadastrada.</p>}
        <ul className="rec-flow-list">
          {list.map((f) =>
            typeof editing === 'object' && editing?.id === f.id ? (
              <li key={f.id}>
                <FlowFormView initial={flowToForm(f)} editing busy={save.isPending} serverError={save.error?.message ?? null}
                  onSubmit={(i) => save.mutate(i)} onCancel={() => setEditing(null)} />
              </li>
            ) : (
              <li key={f.id} className="rec-flow">
                {f.in ? <ArrowDownLeft size={16} aria-label="entrada" /> : <ArrowUpRight size={16} aria-label="saída" />}
                <div>
                  <strong>{f.name}</strong>
                  {f.party && <span className="tile-note"> · {f.party}</span>}
                  <p className="tile-note">
                    {flowRule(f, fmt)}
                    {f.installments > 0 && ` · ${f.done} de ${f.installments} ${f.in ? 'recebidas' : 'pagas'}, falta ${fmt(f.remaining)}`}
                  </p>
                  <p className="tile-note">
                    {f.thisMonthDone ? (
                      <><CircleCheck size={14} aria-hidden /> {f.in ? 'Recebido' : 'Pago'} este mês</>
                    ) : f.nextDate ? (
                      <><Clock size={14} aria-hidden /> Próxima {shortDay(f.nextDate)}</>
                    ) : null}
                    {!f.payeeMatch && ' · sem nome no extrato, o app não reconhece quando acontece'}
                  </p>
                </div>
                <span className="rec-flow-amount">
                  {fmt(f.active ? f.thisMonth : f.nextAmount)}
                </span>
                <span className="rec-actions">
                  <button type="button" className="btn btn-icon" aria-label={`Editar ${f.name}`} onClick={() => { save.reset(); setEditing(f) }}>
                    <Pencil size={16} />
                  </button>
                  <button type="button" className="btn btn-icon" aria-label={`Apagar ${f.name}`} disabled={remove.isPending}
                    onClick={() => window.confirm(`Apagar "${f.name}"?`) && remove.mutate(f.id)}>
                    <Trash2 size={16} />
                  </button>
                </span>
              </li>
            ),
          )}
        </ul>
        {editing === (isIn ? 'in' : 'out') ? (
          <FlowFormView initial={emptyFlowForm(isIn, startMonth)} editing={false} busy={save.isPending} serverError={save.error?.message ?? null}
            onSubmit={(i) => save.mutate(i)} onCancel={() => setEditing(null)} />
        ) : (
          <button type="button" className="btn" onClick={() => open(isIn ? 'in' : 'out')}>
            <Plus size={16} aria-hidden /> {isIn ? 'Nova entrada' : 'Nova saída'}
          </button>
        )}
      </div>
    )
  }

  return (
    <section className="rec-card" aria-labelledby="rec-flows-title">
      <div className="rec-head">
        <h2 id="rec-flows-title">Recorrências</h2>
        <p className="tile-note">
          O que entra e sai todo mês fora o salário. O que ainda não caiu no banco entra em &quot;a receber&quot; e &quot;a pagar&quot;.
        </p>
      </div>
      {query.isError && <p className="state error">Não foi possível carregar as recorrências.</p>}
      {query.data && (
        <div className="rec-flow-groups">
          {group(true)}
          {group(false)}
        </div>
      )}
    </section>
  )
}

interface FormProps {
  initial: FlowForm
  editing: boolean
  busy: boolean
  serverError: string | null
  onSubmit: (input: FlowInput) => void
  onCancel: () => void
}

function FlowFormView({ initial, editing, busy, serverError, onSubmit, onCancel }: FormProps) {
  const [f, setF] = useState<FlowForm>(initial)
  const [error, setError] = useState<string | null>(null)
  const set = (c: Partial<FlowForm>) => setF((p) => ({ ...p, ...c }))
  function submit(e: FormEvent) {
    e.preventDefault()
    const r = toFlowInput(f)
    if ('error' in r) setError(r.error)
    else {
      setError(null)
      onSubmit(r.input)
    }
  }
  return (
    <form className="rec-form" onSubmit={submit} aria-label={editing ? 'Editar recorrência' : 'Nova recorrência'}>
      <div className="view-toggle" role="group" aria-label="Sentido">
        <button type="button" className="btn" aria-pressed={f.in} onClick={() => set({ in: true })}>Entrada</button>
        <button type="button" className="btn" aria-pressed={!f.in} onClick={() => set({ in: false })}>Saída</button>
      </div>
      <div className="form-grid">
        <label>
          Nome
          <input type="text" maxLength={60} required placeholder={f.in ? 'Ex.: Desmame do aluguel' : 'Ex.: Aluguel'} value={f.name}
            onChange={(e) => set({ name: e.target.value })} />
        </label>
        <label>
          Com quem
          <input type="text" maxLength={60} placeholder="Ex.: Mãe, irmão" value={f.party} onChange={(e) => set({ party: e.target.value })} />
        </label>
        <label>
          Valor (R$){f.step ? ' no 1º mês' : ''}
          <input type="text" inputMode="decimal" required value={f.amount} onChange={(e) => set({ amount: e.target.value })} />
        </label>
        <label>
          Dia
          <input type="number" min={1} max={31} required value={f.day} onChange={(e) => set({ day: e.target.value })} />
        </label>
        <label>
          Começa em
          <input type="month" required value={f.startMonth} onChange={(e) => set({ startMonth: e.target.value })} />
        </label>
        <label>
          Quantas vezes (vazio = sem fim)
          <input type="number" min={1} max={120} value={f.installments} onChange={(e) => set({ installments: e.target.value })} />
        </label>
        <label>
          Sobe por mês (R$, opcional)
          <input type="text" inputMode="decimal" value={f.step} onChange={(e) => set({ step: e.target.value })} />
        </label>
        <label>
          Até no máximo (R$, opcional)
          <input type="text" inputMode="decimal" value={f.maxAmount} onChange={(e) => set({ maxAmount: e.target.value })} />
        </label>
        <label>
          Nome no extrato
          <input type="text" maxLength={200} placeholder="Separe com ; (ex.: Maria Silva; Maria S Souza)" value={f.payeeMatch}
            onChange={(e) => set({ payeeMatch: e.target.value })} />
        </label>
      </div>
      {(error ?? serverError) && (
        <p className="state error" role="alert">
          {error ?? serverError}
        </p>
      )}
      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {editing ? 'Salvar' : 'Adicionar'}
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  )
}
