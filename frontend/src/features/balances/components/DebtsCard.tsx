import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, CalendarClock, CircleCheck, HandCoins, Pencil, Plus, Trash2 } from 'lucide-react'
import { formatMonthShort, shortDay } from '../../../shared/lib/format'
import { createDebt, deleteDebt, updateDebt, useDebts, type Debt, type DebtInput } from '../api'
import { debtOutstanding, debtToForm, emptyDebtForm, nextDueDate } from '../lib/debts'
import { DebtForm } from './DebtForm'
import { StaticMoney } from './Money'

interface Props {
  hidden: boolean
  accounts: { id: string; label: string }[]
}

/** As dívidas pessoais: as parceladas e a coberta pelo cheque especial, com a previsão do próximo mergulho no limite. */
export function DebtsCard({ hidden, accounts }: Props) {
  const query = useDebts()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<Debt | 'new' | null>(null)
  const [firstDue, setFirstDue] = useState('')
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['debts'] })
    queryClient.invalidateQueries({ queryKey: ['balances'] })
  }
  const save = useMutation({
    mutationFn: (input: DebtInput) => (editing && editing !== 'new' ? updateDebt(editing.id, input) : createDebt(input)),
    onSuccess: () => {
      setEditing(null)
      refresh()
    },
  })
  const remove = useMutation({ mutationFn: deleteDebt, onSuccess: refresh })

  const debts = query.data?.debts ?? []
  const total = debts.reduce((s, d) => s + debtOutstanding(d), 0)

  return (
    <section className="bal-salary bal-debts" aria-labelledby="bal-debts-title">
      <div className="bal-salary-head">
        <h2 id="bal-debts-title">Dívidas</h2>
        {debts.length > 0 && (
          <p>
            Em aberto <strong><StaticMoney value={total} hidden={hidden} /></strong>
          </p>
        )}
      </div>

      {query.isError && <p className="state error">Não foi possível carregar as dívidas.</p>}
      {query.data && debts.length === 0 && editing === null && (
        <p className="bal-aside">Nenhuma dívida cadastrada. Cadastre uma ajuda da família coberta pelo limite ou uma dívida parcelada.</p>
      )}

      <ul className="bal-debt-list">
        {debts.map((d) =>
          editing !== 'new' && editing?.id === d.id ? (
            <li key={d.id}>
              <DebtForm initial={debtToForm(d)} editing accounts={accounts} busy={save.isPending} serverError={save.error?.message ?? null}
                onSubmit={(input) => save.mutate(input)} onCancel={() => setEditing(null)} />
            </li>
          ) : (
            <li key={d.id} className="bal-debt">
              <div className="bal-debt-head">
                <HandCoins size={18} aria-hidden />
                <div>
                  <h3>{d.name}</h3>
                  <p className="tile-note">
                    {[d.creditor && `com ${d.creditor}`, d.note].filter(Boolean).join(' · ')}
                  </p>
                </div>
                <div className="bal-debt-actions">
                  <button type="button" className="btn btn-icon" aria-label={`Editar ${d.name}`} onClick={() => setEditing(d)}>
                    <Pencil size={16} />
                  </button>
                  <button type="button" className="btn btn-icon" aria-label={`Apagar ${d.name}`} disabled={remove.isPending}
                    onClick={() => window.confirm(`Apagar a dívida "${d.name}"?`) && remove.mutate(d.id)}>
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
              {d.kind === 'INSTALLMENTS' ? <Installments debt={d} hidden={hidden} /> : <Overdraft debt={d} hidden={hidden} />}
            </li>
          ),
        )}
      </ul>

      {editing === 'new' ? (
        <DebtForm initial={emptyDebtForm('OVERDRAFT', firstDue)} editing={false} accounts={accounts}
          busy={save.isPending} serverError={save.error?.message ?? null} onSubmit={(input) => save.mutate(input)} onCancel={() => setEditing(null)} />
      ) : (
        <button type="button" className="btn" onClick={() => { save.reset(); setFirstDue(nextDueDate(new Date(), 10)); setEditing('new') }}>
          <Plus size={16} aria-hidden /> Nova dívida
        </button>
      )}
    </section>
  )
}

function Installments({ debt: d, hidden }: { debt: Debt; hidden: boolean }) {
  const p = d.progress
  if (!p) return null
  const ratio = p.total > 0 ? p.paid / p.total : 0
  return (
    <div className="bal-debt-body">
      <div className="bal-debt-bar" role="progressbar" aria-valuemin={0} aria-valuemax={p.total} aria-valuenow={p.paid}
        aria-label={`${p.paid} de ${p.total} parcelas pagas`}>
        <span style={{ width: `${Math.round(ratio * 100)}%` }} />
      </div>
      <p>
        <strong>{p.paid} de {p.total}</strong> parcelas de <StaticMoney value={d.installmentAmount} hidden={hidden} /> pagas · falta{' '}
        <strong><StaticMoney value={p.remaining} hidden={hidden} /></strong>
      </p>
      {p.done ? (
        <p className="bal-debt-ok"><CircleCheck size={16} aria-hidden /> Quitada</p>
      ) : p.nextDue && (
        <p className={p.overdue ? 'bal-debt-warn' : undefined}>
          {p.overdue ? <AlertTriangle size={16} aria-hidden /> : <CalendarClock size={16} aria-hidden />}{' '}
          {p.overdue ? 'Parcela atrasada desde' : 'Próxima parcela'} {shortDay(p.nextDue)}
          {p.overdue && !d.payeeMatch && ' (preencha o nome no Pix para o app reconhecer o pagamento)'}
        </p>
      )}
      {!d.payeeMatch && !p.overdue && !p.done && (
        <p className="tile-note">Preencha o nome de quem recebe no Pix (em Editar) para as parcelas pagas contarem sozinhas.</p>
      )}
      {p.payments.length > 0 && (
        <p className="tile-note">
          Último Pix reconhecido: {shortDay(p.payments[0].date)}, <StaticMoney value={p.payments[0].amount} hidden={hidden} />
        </p>
      )}
    </div>
  )
}

function Overdraft({ debt: d, hidden }: { debt: Debt; hidden: boolean }) {
  const l = d.limit
  const f = d.forecast
  if (!l) {
    return <p className="tile-note">A conta ligada a esta dívida não está mais sincronizada. Edite e escolha outra.</p>
  }
  const months = l.months.filter((m) => m.days > 0).slice(0, 3)
  return (
    <div className="bal-debt-body">
      <p className="tile-note">
        Ajuda de <StaticMoney value={d.originalAmount} hidden={hidden} />, coberta pelo limite da conta {d.accountName}. O negativo dessa
        conta aparece aqui, não no saldo.
      </p>
      {l.inLimit > 0 ? (
        <p className={l.daysUsed > d.freeDays ? 'bal-debt-warn' : undefined}>
          {l.daysUsed > d.freeDays ? <AlertTriangle size={16} aria-hidden /> : <CalendarClock size={16} aria-hidden />}{' '}
          <strong><StaticMoney value={l.inLimit} hidden={hidden} /></strong> no limite agora · dia {l.daysUsed} de {d.freeDays} sem juros
          {l.since && ` (desde ${shortDay(l.since)})`}
          {l.daysUsed > d.freeDays && ' · já passou dos dias sem juros'}
        </p>
      ) : (
        <p className="bal-debt-ok"><CircleCheck size={16} aria-hidden /> Coberta agora: a conta está positiva.</p>
      )}

      {f && (
        <div className="bal-debt-forecast">
          {f.inLimit > 0 ? (
            <p>
              Previsão para <strong>{shortDay(f.date)}</strong>: cerca de <strong><StaticMoney value={f.inLimit} hidden={hidden} /></strong> no
              limite
              {f.exitDate ? (
                <>
                  {' '}por {f.days} {f.days === 1 ? 'dia' : 'dias'}, até o {f.exitLabel.toLowerCase()} ({shortDay(f.exitDate)}).{' '}
                  {f.withinFree ? 'Dentro dos dias sem juros (só IOF).' : <strong>Passa dos {d.freeDays} dias sem juros.</strong>}
                </>
              ) : (
                '. O salário configurado não cobre: confira as partes do salário em Configurações.'
              )}
            </p>
          ) : (
            <p>Previsão para <strong>{shortDay(f.date)}</strong>: a conta não deve entrar no limite.</p>
          )}
          <details>
            <summary>Como chegamos nesse número</summary>
            <ul className="bal-debt-lines">
              <li>
                <span>Saldo da conta hoje</span>
                <StaticMoney value={f.balance - f.lines.reduce((s, x) => s + x.amount, 0)} hidden={hidden} />
              </li>
              {f.lines.map((x, n) => (
                <li key={`${x.label}-${n}`}>
                  <span>
                    {x.label}
                    {x.date && ` · ${shortDay(x.date)}`}
                  </span>
                  <StaticMoney value={x.amount} hidden={hidden} />
                </li>
              ))}
              <li className="is-total">
                <span>Saldo previsto em {shortDay(f.date)}</span>
                <StaticMoney value={f.balance} hidden={hidden} />
              </li>
            </ul>
            <p className="tile-note">
              A fatura soma o que já entrou nela até agora (compras novas aumentam). As saídas de costume repetem o que saiu da conta no
              mesmo período do mês passado.
            </p>
          </details>
        </div>
      )}

      {months.length > 0 && (
        <p className="tile-note">
          Histórico no limite:{' '}
          {months.map((m) => `${formatMonthShort(m.month)} ${m.days} ${m.days === 1 ? 'dia' : 'dias'}`).join(' · ')}
          {months[0] && (
            <>
              {' '}(pior: <StaticMoney value={months[0].worst} hidden={hidden} /> em {formatMonthShort(months[0].month)})
            </>
          )}
        </p>
      )}
    </div>
  )
}
