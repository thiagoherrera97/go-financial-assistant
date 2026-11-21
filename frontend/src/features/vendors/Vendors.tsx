import './styles.css'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, CircleAlert, Link2, Pencil, Plus, Settings2, Trash2, TriangleAlert, X } from 'lucide-react'
import { useState, type CSSProperties } from 'react'
import { QueryState } from '../../shared/components/QueryState'
import { StatTile } from '../../shared/components/StatTile'
import { ApiError } from '../../shared/api/request'
import { formatBRL, formatPercent } from '../../shared/lib/format'
import type { Tone } from '../../shared/lib/tone'
import { PaymentPanel } from './components/PaymentPanel'
import { PlanForm } from './components/PlanForm'
import { VendorForm } from './components/VendorForm'
import { emptyVendorForm, formatDate, linkedIds, paidRatio, planStatus, vendorToForm } from './lib/vendors'
import {
  VENDORS_KEY,
  addPayment,
  createVendor,
  deletePayment,
  deleteVendor,
  savePlan,
  updateVendor,
  useVendors,
  type Vendor,
  type VendorInput,
} from './api'

const TONE_ICON = { ok: Check, warning: TriangleAlert, critical: CircleAlert }

const errorText = (e: unknown, fallback: string) => (e instanceof ApiError ? e.message : e ? fallback : null)

export default function Vendors() {
  const vendors = useVendors()
  const queryClient = useQueryClient()
  const refresh = () => queryClient.invalidateQueries({ queryKey: VENDORS_KEY })
  const [adding, setAdding] = useState(false)
  const [editingPlan, setEditingPlan] = useState(false)

  const create = useMutation({
    mutationFn: createVendor,
    onSuccess: () => {
      setAdding(false)
      return refresh()
    },
  })
  const plan = useMutation({
    mutationFn: savePlan,
    onSuccess: () => {
      setEditingPlan(false)
      return refresh()
    },
  })

  return (
    <QueryState query={vendors}>
      {(s) => {
        const status = planStatus(s)
        const StatusIcon = TONE_ICON[status.tone]
        const linked = linkedIds(s.vendors)
        const savedNote = s.plan.savingsBox
          ? s.savedFromBox
            ? `Caixinha “${s.plan.savingsBox}”.`
            : `Caixinha “${s.plan.savingsBox}” não encontrada nas contas.`
          : 'Informado à mão.'
        return (
          <>
            <div className="vendors-head">
              <h1 className="page-title">{s.plan.name}</h1>
              {!editingPlan && (
                <button type="button" className="btn btn-small" onClick={() => setEditingPlan(true)}>
                  <Settings2 size={13} aria-hidden="true" /> Ajustar plano
                </button>
              )}
            </div>

            {editingPlan && (
              <div className="section-gap">
                <PlanForm
                  plan={s.plan}
                  boxes={s.boxes}
                  busy={plan.isPending}
                  serverError={errorText(plan.error, 'Não foi possível salvar o plano.')}
                  onSubmit={(p) => plan.mutate(p)}
                  onCancel={() => {
                    setEditingPlan(false)
                    plan.reset()
                  }}
                />
              </div>
            )}

            <div className="tiles vendor-tiles section-gap">
              <StatTile label="Contratado" value={formatBRL(s.contracted)} note={`${s.vendors.length} fornecedores`} />
              <StatTile label="Pago" value={formatBRL(s.paid)} note={s.contracted > 0 ? `${formatPercent(s.paid / s.contracted)} do contratado` : undefined} />
              <StatTile label="Guardado" value={formatBRL(s.saved)} note={savedNote} />
              <StatTile label="Falta pagar" value={formatBRL(s.remaining)} note="Contratado − pago − guardado." />
              <StatTile
                label="Guardar por mês"
                value={formatBRL(s.perMonth)}
                note={
                  s.lastDue
                    ? `${s.monthsLeft} aportes, de ${formatDate(s.nextContribution)} até ${formatDate(s.lastDue)}.`
                    : `Próximo aporte: ${formatDate(s.nextContribution)}.`
                }
              />
            </div>

            <p className={`vendor-status vendor-${status.tone} section-gap`}>
              <StatusIcon size={14} aria-hidden="true" /> {status.text}
            </p>
            {s.tight && (
              <p className="vendor-status vendor-warning">
                <TriangleAlert size={14} aria-hidden="true" /> O prazo de {s.tight} pede {formatBRL(s.perMonth)} por mês, mais que a média de{' '}
                {formatBRL(s.flatPerMonth)} (o que falta ÷ aportes até o último vencimento).
              </p>
            )}

            {s.vendors.length === 0 && !adding && <p className="state">Nenhum fornecedor ainda.</p>}
            <div className="bill-grid section-gap">
              {s.vendors.map((v) => (
                <VendorCard key={v.id} v={v} linked={linked} onChange={refresh} />
              ))}
            </div>

            {adding ? (
              <div className="section-gap">
                <VendorForm
                  initial={emptyVendorForm()}
                  creating
                  busy={create.isPending}
                  serverError={errorText(create.error, 'Não foi possível adicionar o fornecedor.')}
                  onSubmit={(input) => create.mutate(input)}
                  onCancel={() => {
                    setAdding(false)
                    create.reset()
                  }}
                />
              </div>
            ) : (
              <button type="button" className="btn btn-primary section-gap" onClick={() => setAdding(true)}>
                <Plus size={14} aria-hidden="true" /> Novo fornecedor
              </button>
            )}

            <p className="tile-note section-gap">
              Guardar por mês divide o que falta pelos aportes (todo dia {s.plan.contributionDay}) até cada vencimento e fica com o maior, para
              que nenhum fornecedor vença sem o dinheiro. O guardado paga primeiro quem vence antes. Pagamentos ligados ao extrato usam o valor e a
              data da transação; se ela for apagada, o pagamento some junto. &quot;Cabe na sobra&quot; usa a sobra média da Projeção, que é uma
              estimativa.
            </p>
          </>
        )
      }}
    </QueryState>
  )
}

function VendorCard({ v, linked, onChange }: { v: Vendor; linked: Set<string>; onChange: () => Promise<unknown> }) {
  const [editing, setEditing] = useState(false)
  const [paying, setPaying] = useState(false)

  const update = useMutation({
    mutationFn: (input: VendorInput) => updateVendor(v.id, input),
    onSuccess: () => {
      setEditing(false)
      return onChange()
    },
  })
  const remove = useMutation({ mutationFn: () => deleteVendor(v.id), onSuccess: onChange })
  const pay = useMutation({
    mutationFn: (p: Parameters<typeof addPayment>[1]) => addPayment(v.id, p),
    onSuccess: () => {
      setPaying(false)
      return onChange()
    },
  })
  const unpay = useMutation({ mutationFn: (id: string) => deletePayment(v.id, id), onSuccess: onChange })

  if (editing)
    return (
      <VendorForm
        initial={vendorToForm(v)}
        creating={false}
        busy={update.isPending}
        serverError={errorText(update.error, 'Não foi possível salvar.')}
        onSubmit={(input) => update.mutate(input)}
        onCancel={() => {
          setEditing(false)
          update.reset()
        }}
      />
    )

  const ratio = paidRatio(v)
  const level: Tone = v.done ? 'ok' : v.late ? 'critical' : 'ok'
  const due = v.dueDate
    ? v.done
      ? `Quitado · prazo ${formatDate(v.dueDate)}`
      : v.late
        ? `Vence ${formatDate(v.dueDate)}, antes do próximo aporte`
        : `Até ${formatDate(v.dueDate)} · ${v.contributions} ${v.contributions === 1 ? 'aporte' : 'aportes'}`
    : v.done
      ? 'Quitado'
      : 'Sem data fim'
  const caption = `${formatBRL(v.paid)} de ${formatBRL(v.contracted)} (${formatPercent(ratio)})`

  return (
    <article className="bill" style={{ '--c': v.late ? 'var(--critical)' : 'var(--accent)' } as CSSProperties}>
      <div className="bill-top">
        <span className="bill-name">
          <strong>{v.name}</strong>
          <span className="bill-meta">{due}</span>
        </span>
        <span className="bill-amount">{v.done ? <Check size={16} aria-label="quitado" /> : `falta ${formatBRL(v.remaining)}`}</span>
      </div>
      <div className="meter" role="meter" aria-label="Quanto já foi pago" aria-valuemin={0} aria-valuemax={v.contracted} aria-valuenow={Math.min(v.paid, v.contracted)} aria-valuetext={caption}>
        <div className={`meter-fill meter-${level}`} style={{ width: `${ratio * 100}%` }} />
      </div>
      <p className="tile-note">{caption}</p>

      {v.payments.length > 0 && (
        <ul className="vendor-payments" aria-label={`Pagamentos a ${v.name}`}>
          {v.payments.map((p) => (
            <li key={p.id}>
              <span>
                {formatDate(p.paidOn)} · <strong>{formatBRL(p.amount)}</strong>{' '}
                {p.transactionId ? (
                  <span className="bill-meta" title={p.description}>
                    <Link2 size={11} aria-label="do extrato" /> {p.description || 'extrato'}
                  </span>
                ) : (
                  <span className="bill-meta">à mão</span>
                )}
              </span>
              <button
                type="button"
                className="icon-btn"
                aria-label={`Remover pagamento de ${formatBRL(p.amount)} em ${formatDate(p.paidOn)}`}
                disabled={unpay.isPending}
                onClick={() => window.confirm(`Remover o pagamento de ${formatBRL(p.amount)}?`) && unpay.mutate(p.id)}
              >
                <X size={13} aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {(remove.isError || unpay.isError) && (
        <p className="state error" role="alert">
          Não foi possível apagar. Tente de novo.
        </p>
      )}

      {paying ? (
        <PaymentPanel
          vendorName={v.name}
          linked={linked}
          busy={pay.isPending}
          serverError={errorText(pay.error, 'Não foi possível lançar o pagamento.')}
          onManual={(p) => pay.mutate(p)}
          onLink={(transactionId) => pay.mutate({ transactionId })}
          onClose={() => {
            setPaying(false)
            pay.reset()
          }}
        />
      ) : (
        <div className="bill-foot">
          <button type="button" className="btn btn-small vendor-first" onClick={() => setPaying(true)}>
            <Plus size={13} aria-hidden="true" /> Pagamento
          </button>
          <button type="button" className="btn btn-small" onClick={() => setEditing(true)}>
            <Pencil size={13} aria-hidden="true" /> Editar
          </button>
          <button
            type="button"
            className="btn btn-small"
            disabled={remove.isPending}
            onClick={() => window.confirm(`Apagar o fornecedor "${v.name}" e os pagamentos dele?`) && remove.mutate()}
          >
            <Trash2 size={13} aria-hidden="true" /> Apagar
          </button>
        </div>
      )}
    </article>
  )
}
