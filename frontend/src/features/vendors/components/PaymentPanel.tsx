import { useQuery } from '@tanstack/react-query'
import { Link2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { formatBRL } from '../../../shared/lib/format'
import { formatDate, isOutflow, parseAmount, todayISO } from '../lib/vendors'
import { searchTransactions } from '../api'

interface Props {
  vendorName: string
  /** Transações já ligadas a algum fornecedor (não aparecem para ligar de novo). */
  linked: Set<string>
  busy: boolean
  serverError: string | null
  onManual: (p: { amount: number; paidOn: string }) => void
  onLink: (transactionId: string) => void
  onClose: () => void
}

/** Lança um pagamento: ligando uma saída do extrato (o padrão) ou à mão. */
export function PaymentPanel({ vendorName, linked, busy, serverError, onManual, onLink, onClose }: Props) {
  const [mode, setMode] = useState<'link' | 'manual'>('link')
  const [q, setQ] = useState(vendorName.split(' ')[0])
  const [term, setTerm] = useState(q)
  const [amount, setAmount] = useState('')
  const [paidOn, setPaidOn] = useState(todayISO())
  const [error, setError] = useState<string | null>(null)

  const hits = useQuery({
    queryKey: ['vendor-tx-search', term],
    queryFn: () => searchTransactions(term),
    enabled: mode === 'link' && term.trim().length >= 2,
  })
  const candidates = (hits.data ?? []).filter((t) => isOutflow(t) && !linked.has(t.id))

  function search(e: FormEvent) {
    e.preventDefault()
    setTerm(q.trim())
  }

  function manual(e: FormEvent) {
    e.preventDefault()
    const v = parseAmount(amount)
    if (!(v > 0)) return setError('Informe o valor pago.')
    if (!paidOn) return setError('Informe a data.')
    setError(null)
    onManual({ amount: v, paidOn })
  }

  return (
    <div className="vendor-pay">
      <div className="view-toggle" role="group" aria-label="Como lançar o pagamento">
        <button type="button" className="btn btn-small" aria-pressed={mode === 'link'} onClick={() => setMode('link')}>
          Do extrato
        </button>
        <button type="button" className="btn btn-small" aria-pressed={mode === 'manual'} onClick={() => setMode('manual')}>
          À mão
        </button>
      </div>

      {mode === 'link' ? (
        <>
          <form className="vendor-search" onSubmit={search} role="search">
            <input type="search" aria-label="Buscar no extrato" placeholder="Descrição ou nota" value={q} onChange={(e) => setQ(e.target.value)} />
            <button type="submit" className="btn btn-small">
              Buscar
            </button>
          </form>
          {term.trim().length < 2 ? (
            <p className="tile-note">Digite ao menos 2 letras.</p>
          ) : hits.isPending ? (
            <p className="tile-note">Buscando…</p>
          ) : hits.isError ? (
            <p className="tile-note">Não foi possível buscar no extrato.</p>
          ) : candidates.length === 0 ? (
            <p className="tile-note">Nenhuma saída encontrada para “{term}”. Tente outro trecho da descrição ou lance à mão.</p>
          ) : (
            <ul className="vendor-hits">
              {candidates.map((t) => (
                <li key={t.id}>
                  <span className="vendor-hit-text">
                    <span>
                      <strong>{formatBRL(t.amount)}</strong> · {formatDate(t.date)}
                    </span>
                    <span className="bill-meta">
                      {t.note || t.description}
                      {t.accountName && ` · ${t.accountName}`}
                    </span>
                  </span>
                  <button type="button" className="btn btn-small" disabled={busy} onClick={() => onLink(t.id)}>
                    <Link2 size={13} aria-hidden="true" /> Ligar
                  </button>
                </li>
              ))}
            </ul>
          )}
        </>
      ) : (
        <form className="vendor-manual" onSubmit={manual}>
          <label>
            Valor (R$)
            <input type="text" inputMode="decimal" required value={amount} onChange={(e) => setAmount(e.target.value)} />
          </label>
          <label>
            Data
            <input type="date" required value={paidOn} onChange={(e) => setPaidOn(e.target.value)} />
          </label>
          <button type="submit" className="btn btn-primary btn-small" disabled={busy}>
            Lançar
          </button>
        </form>
      )}
      {(error ?? serverError) && (
        <p className="state error" role="alert">
          {error ?? serverError}
        </p>
      )}
      <button type="button" className="btn btn-small vendor-close" onClick={onClose}>
        Fechar
      </button>
    </div>
  )
}
