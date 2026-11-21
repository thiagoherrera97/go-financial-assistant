import { useState } from 'react'
import { QueryState } from '../../../shared/components/QueryState'
import { StatTile } from '../../../shared/components/StatTile'
import { formatBRL, formatMonthLong } from '../../../shared/lib/format'
import { currentMonth } from '../../../shared/lib/months'
import { useMonthSheet } from '../../month/api'
import { useDebts, type Debt } from '../api'
import { debtLabel, debtRemaining, loadGoal, saveGoal, spendable, type DebtGoal } from '../lib/spendable'

/** A dívida escolhida; sem escolha salva, a primeira em aberto com a mãe. debtId '' é "digitar outra". */
function pickDebt(debts: Debt[], goal: DebtGoal): Debt | undefined {
  if (goal.debtId !== undefined) return debts.find((d) => d.id === goal.debtId)
  const open = debts.filter((d) => debtRemaining(d) > 0)
  return open.find((d) => /m[aã]e/i.test(`${d.creditor} ${d.name}`)) ?? open[0]
}

/** Quanto ainda dá para gastar neste mês para fechar no zero a zero e para sobrar a meta de abatimento da dívida. */
export function SpendableCard({ averageFixed }: { averageFixed: number }) {
  const month = currentMonth()
  const query = useMonthSheet(month)
  const debts = useDebts().data ?? []
  const [goal, setGoal] = useState<DebtGoal>(loadGoal)
  const update = (g: DebtGoal) => {
    setGoal(g)
    saveGoal(g)
  }

  return (
    <section className="card section-gap" aria-labelledby="spend-title">
      <h2 className="chart-title" id="spend-title">
        Quanto ainda dá para gastar em {formatMonthLong(month)}
      </h2>
      <QueryState query={query}>
        {(sheet) => {
          const s = spendable(sheet, averageFixed, goal.amount, new Date())
          const debt = pickDebt(debts, goal)
          const target = debt ? debtLabel(debt) : goal.label.trim() || 'dívida'
          const remaining = debt ? debtRemaining(debt) : null
          return (
            <>
              <div className="tiles">
                <StatTile
                  label="Para fechar no zero a zero"
                  value={s.toBreakEven >= 0 ? formatBRL(s.toBreakEven) : formatBRL(0)}
                  note={
                    s.toBreakEven >= 0
                      ? `Cerca de ${formatBRL(s.perDayBreakEven)} por dia nos ${s.daysLeft} dias que faltam`
                      : `Já vai faltar ${formatBRL(-s.toBreakEven)} mesmo sem gastar mais nada`
                  }
                />
                <StatTile
                  label={`Para abater ${formatBRL(goal.amount)} da ${target}`}
                  value={s.toReachGoal >= 0 ? formatBRL(s.toReachGoal) : formatBRL(0)}
                  note={
                    s.toReachGoal >= 0
                      ? `Cerca de ${formatBRL(s.perDayGoal)} por dia; o que sobrar no fim do mês vai para a ${target}`
                      : `Não dá neste mês: faltam ${formatBRL(-s.toReachGoal)}. Dá para abater até ${formatBRL(Math.max(0, s.toBreakEven))}`
                  }
                />
              </div>
              <p className="tile-note">
                Conta: sobra do mês com o salário a cair ({formatBRL(sheet.leftoverWithPending)})
                {s.fixedToCome > 0 && <> menos as contas fixas que ainda devem cair ({formatBRL(s.fixedToCome)}, pela média)</>}. Compras no cartão
                podem entrar só na fatura do mês que vem.
              </p>
              {remaining !== null && (
                <p className="tile-note">
                  {remaining > 0
                    ? `Faltam ${formatBRL(remaining)} da ${target}` +
                      (goal.amount > 0 ? `: abatendo ${formatBRL(goal.amount)} por mês, quita em cerca de ${Math.ceil(remaining / goal.amount)} ${Math.ceil(remaining / goal.amount) === 1 ? 'mês' : 'meses'}.` : '.')
                    : `A ${target} já está quitada.`}
                </p>
              )}
              <div className="form-grid">
                {debts.length > 0 && (
                  <label>
                    Dívida
                    <select value={debt?.id ?? ''} onChange={(e) => update({ ...goal, debtId: e.target.value })}>
                      {debts.map((d) => (
                        <option key={d.id} value={d.id}>
                          {d.name}
                          {d.creditor ? ` (${d.creditor})` : ''}
                        </option>
                      ))}
                      <option value="">Outra (digitar)</option>
                    </select>
                  </label>
                )}
                {!debt && (
                  <label>
                    Abater da
                    <input type="text" maxLength={60} value={goal.label} onChange={(e) => update({ ...goal, label: e.target.value })} />
                  </label>
                )}
                <label>
                  Quanto (R$)
                  <input
                    type="number"
                    min={0}
                    step="50"
                    value={goal.amount}
                    onChange={(e) => update({ ...goal, amount: Math.max(0, Number(e.target.value) || 0) })}
                  />
                </label>
              </div>
            </>
          )
        }}
      </QueryState>
    </section>
  )
}
