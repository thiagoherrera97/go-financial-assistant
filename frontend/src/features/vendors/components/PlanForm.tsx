import { useState, type FormEvent } from 'react'
import { parseAmount } from '../lib/vendors'
import type { VendorPlan } from '../api'

interface Props {
  plan: VendorPlan
  boxes: string[]
  busy: boolean
  serverError: string | null
  onSubmit: (p: VendorPlan) => void
  onCancel: () => void
}

/** Nome do evento, dia do aporte e onde está o dinheiro guardado (caixinha ou valor à mão). */
export function PlanForm({ plan, boxes, busy, serverError, onSubmit, onCancel }: Props) {
  const [name, setName] = useState(plan.name)
  const [day, setDay] = useState(String(plan.contributionDay))
  const [box, setBox] = useState(plan.savingsBox)
  const [manual, setManual] = useState(plan.savedManual ? String(plan.savedManual).replace('.', ',') : '')
  const [error, setError] = useState<string | null>(null)
  // A caixinha gravada pode ter sumido das contas; continua na lista para não ser trocada sem querer.
  const options = plan.savingsBox && !boxes.includes(plan.savingsBox) ? [...boxes, plan.savingsBox] : boxes

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const d = Number(day)
    const saved = manual.trim() ? parseAmount(manual) : 0
    if (!name.trim()) return setError('Informe o nome do evento.')
    if (!Number.isInteger(d) || d < 1 || d > 28) return setError('O dia do aporte vai de 1 a 28.')
    if (!box && !(saved >= 0)) return setError('O valor guardado não é um número válido.')
    setError(null)
    onSubmit({ name: name.trim(), contributionDay: d, savingsBox: box, savedManual: box ? 0 : saved })
  }

  return (
    <form className="card scenario-form" onSubmit={handleSubmit} aria-label="Ajustar plano">
      <h2 className="chart-title">Ajustar plano</h2>
      <div className="form-grid">
        <label>
          Evento
          <input type="text" maxLength={60} required value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label>
          Dia do aporte mensal
          <input type="number" min={1} max={28} step={1} required value={day} onChange={(e) => setDay(e.target.value)} />
        </label>
        <label>
          Dinheiro guardado em
          <select value={box} onChange={(e) => setBox(e.target.value)}>
            <option value="">Informar à mão</option>
            {options.map((b) => (
              <option key={b} value={b}>
                Caixinha “{b}”
              </option>
            ))}
          </select>
        </label>
        {!box && (
          <label>
            Valor guardado (R$)
            <input type="text" inputMode="decimal" placeholder="0,00" value={manual} onChange={(e) => setManual(e.target.value)} />
          </label>
        )}
      </div>
      <p className="tile-note">
        Com uma caixinha, o guardado é o saldo dela na última sincronização.
        {boxes.length === 0 && ' Nenhuma caixinha foi encontrada nas suas contas correntes ainda.'}
      </p>
      {(error ?? serverError) && (
        <p className="state error" role="alert">
          {error ?? serverError}
        </p>
      )}
      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          Salvar
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  )
}
