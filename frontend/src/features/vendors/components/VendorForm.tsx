import { useState, type FormEvent } from 'react'
import { toVendorInput, type VendorForm as Form } from '../lib/vendors'
import type { VendorInput } from '../api'

interface Props {
  initial: Form
  /** Na criação aparece o campo "já pago", que vira o primeiro pagamento. */
  creating: boolean
  busy: boolean
  serverError: string | null
  onSubmit: (input: VendorInput) => void
  onCancel: () => void
}

export function VendorForm({ initial, creating, busy, serverError, onSubmit, onCancel }: Props) {
  const [f, setF] = useState(initial)
  const [error, setError] = useState<string | null>(null)
  const set = (changes: Partial<Form>) => setF((prev) => ({ ...prev, ...changes }))

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const result = toVendorInput(f, creating)
    if ('error' in result) setError(result.error)
    else {
      setError(null)
      onSubmit(result.input)
    }
  }

  const title = creating ? 'Novo fornecedor' : `Editar ${initial.name}`
  return (
    <form className="card scenario-form" onSubmit={handleSubmit} aria-label={title}>
      <h2 className="chart-title">{title}</h2>
      <div className="form-grid">
        <label>
          Fornecedor
          <input type="text" maxLength={60} required placeholder="Ex.: Buffet" value={f.name} onChange={(e) => set({ name: e.target.value })} />
        </label>
        <label>
          Valor contratado (R$)
          <input type="text" inputMode="decimal" required placeholder="18.489,98" value={f.contracted} onChange={(e) => set({ contracted: e.target.value })} />
        </label>
        <label>
          Data fim do pagamento
          <input type="date" value={f.dueDate} onChange={(e) => set({ dueDate: e.target.value })} />
        </label>
        {creating && (
          <label>
            Já pago até hoje (R$)
            <input type="text" inputMode="decimal" placeholder="0,00" value={f.paid} onChange={(e) => set({ paid: e.target.value })} />
          </label>
        )}
      </div>
      {creating && (
        <p className="tile-note">O valor já pago vira um pagamento com a data de hoje. Daqui em diante, ligue os pagamentos do extrato.</p>
      )}
      {(error ?? serverError) && (
        <p className="state error" role="alert">
          {error ?? serverError}
        </p>
      )}
      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {creating ? 'Adicionar' : 'Salvar'}
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  )
}
