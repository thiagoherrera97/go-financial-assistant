import { useState, type FormEvent } from 'react'
import { DEBT_KINDS, toDebtInput, type DebtForm as Form } from '../lib/debts'
import type { DebtInput, DebtKind } from '../api'

interface Props {
  initial: Form
  editing: boolean
  /** Contas correntes que podem cobrir a dívida com o limite. */
  accounts: { id: string; label: string }[]
  busy: boolean
  serverError: string | null
  onSubmit: (input: DebtInput) => void
  onCancel: () => void
}

/** Cadastro e edição de uma dívida; a validação daqui espelha a da API. */
export function DebtForm({ initial, editing, accounts, busy, serverError, onSubmit, onCancel }: Props) {
  const [f, setF] = useState<Form>(initial)
  const [error, setError] = useState<string | null>(null)
  const set = (changes: Partial<Form>) => setF((prev) => ({ ...prev, ...changes }))

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const result = toDebtInput(f)
    if ('error' in result) setError(result.error)
    else {
      setError(null)
      onSubmit(result.input)
    }
  }

  return (
    <form className="bal-debt-form" onSubmit={handleSubmit} aria-label={editing ? 'Editar dívida' : 'Nova dívida'}>
      {!editing && (
        <div className="view-toggle" role="group" aria-label="Tipo de dívida">
          {(Object.keys(DEBT_KINDS) as DebtKind[]).map((k) => (
            <button key={k} type="button" className="btn" aria-pressed={f.kind === k} onClick={() => set({ kind: k })}>
              {DEBT_KINDS[k].label}
            </button>
          ))}
        </div>
      )}
      <p className="tile-note">{DEBT_KINDS[f.kind].hint}.</p>

      <div className="form-grid">
        <label>
          Nome
          <input type="text" maxLength={60} required placeholder={f.kind === 'OVERDRAFT' ? 'Ex.: Ajuda do carro' : 'Ex.: Mac mini'}
            value={f.name} onChange={(e) => set({ name: e.target.value })} />
        </label>
        <label>
          Com quem
          <input type="text" maxLength={60} placeholder="Ex.: Mãe" value={f.creditor} onChange={(e) => set({ creditor: e.target.value })} />
        </label>
        <label>
          Para quê (opcional)
          <input type="text" maxLength={120} placeholder="Ex.: compra do MG4 Urban" value={f.note} onChange={(e) => set({ note: e.target.value })} />
        </label>

        {f.kind === 'OVERDRAFT' ? (
          <>
            <label>
              Valor da ajuda (R$)
              <input type="text" inputMode="decimal" required value={f.amount} onChange={(e) => set({ amount: e.target.value })} />
            </label>
            <label>
              Conta do limite
              <select required value={f.accountId} onChange={(e) => set({ accountId: e.target.value })}>
                <option value="">Escolha…</option>
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Dia em que entra no limite
              <input type="number" min={1} max={31} required value={f.dueDay} onChange={(e) => set({ dueDay: e.target.value })} />
            </label>
            <label>
              Dias sem juros do banco
              <input type="number" min={0} max={60} required value={f.freeDays} onChange={(e) => set({ freeDays: e.target.value })} />
            </label>
          </>
        ) : (
          <>
            <label>
              Valor da parcela (R$)
              <input type="text" inputMode="decimal" required value={f.installmentAmount} onChange={(e) => set({ installmentAmount: e.target.value })} />
            </label>
            <label>
              Parcelas
              <input type="number" min={1} max={120} required value={f.installments} onChange={(e) => set({ installments: e.target.value })} />
            </label>
            <label>
              Primeira parcela
              <input type="date" required value={f.firstDue}
                onChange={(e) => set({ firstDue: e.target.value, dueDay: e.target.value ? String(Number(e.target.value.slice(8))) : f.dueDay })} />
            </label>
            <label>
              Nomes no Pix de quem recebe
              <input type="text" maxLength={200} placeholder="Separe com ; (ex.: Maria Silva; Maria S Souza)" value={f.payeeMatch}
                onChange={(e) => set({ payeeMatch: e.target.value })} />
            </label>
            <label>
              Parcelas pagas fora do Pix
              <input type="number" min={0} max={Number(f.installments) || 120} value={f.paidManual} onChange={(e) => set({ paidManual: e.target.value })} />
            </label>
          </>
        )}
      </div>

      {(error ?? serverError) && (
        <p className="state error" role="alert">
          {error ?? serverError}
        </p>
      )}
      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {editing ? 'Salvar' : 'Criar dívida'}
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  )
}
