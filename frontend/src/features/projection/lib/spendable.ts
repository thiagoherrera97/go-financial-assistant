// "Quanto ainda dá para gastar neste mês": parte do que a tela do mês já mostra (entradas, salário a cair e despesas
// lançadas) e tira as contas fixas que ainda não caíram, estimadas pela média de fixas da projeção.

export interface MonthNumbers {
  /** Entradas menos despesas já lançadas, contando o salário que ainda vai cair. */
  leftoverWithPending: number
  /** Fixas e parcelas fora do cartão já lançadas no mês. */
  fixedTotal: number
  /** Último dia do ciclo do salário (AAAA-MM-DD), quando a API informa. */
  endsOn?: string
}

export interface Spendable {
  /** Fixas que ainda devem cair no mês (média de fixas menos o que já caiu, nunca negativo). */
  fixedToCome: number
  /** Quanto ainda dá para gastar e fechar o mês no zero a zero (negativo = já vai faltar). */
  toBreakEven: number
  /** Quanto dá para gastar e ainda sobrar a meta (ex.: abater a dívida). */
  toReachGoal: number
  /** Dias que faltam no mês, contando hoje. */
  daysLeft: number
  /** Por dia, para fechar no zero e para bater a meta (0 se já não dá). */
  perDayBreakEven: number
  perDayGoal: number
}

/** Dias que faltam no ciclo do salário (até `endsOn`) ou, sem ele, no mês de `today`; contando hoje. */
export function daysLeftInMonth(today: Date, endsOn?: string): number {
  if (endsOn) {
    const [y, m, d] = endsOn.split('-').map(Number)
    const end = Date.UTC(y, m - 1, d)
    const start = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate())
    return Math.max(1, Math.round((end - start) / 86_400_000) + 1)
  }
  const last = new Date(today.getFullYear(), today.getMonth() + 1, 0).getDate()
  return last - today.getDate() + 1
}

export function spendable(month: MonthNumbers, averageFixed: number, goal: number, today: Date): Spendable {
  const fixedToCome = Math.max(0, averageFixed - month.fixedTotal)
  const toBreakEven = month.leftoverWithPending - fixedToCome
  const toReachGoal = toBreakEven - Math.max(0, goal)
  const daysLeft = daysLeftInMonth(today, month.endsOn)
  return {
    fixedToCome,
    toBreakEven,
    toReachGoal,
    daysLeft,
    perDayBreakEven: Math.max(0, toBreakEven) / daysLeft,
    perDayGoal: Math.max(0, toReachGoal) / daysLeft,
  }
}

// --- Meta de abatimento (só no navegador) ---

export interface DebtGoal {
  /** Para onde vai a sobra (ex.: "dívida com a mãe"). */
  label: string
  amount: number
  /** Dívida cadastrada escolhida; sem ela, vale o nome digitado. */
  debtId?: string
}

/**
 * Quanto falta de uma dívida, pela mesma regra do card de dívidas (balances/lib/debts.ts, debtOutstanding): o que resta
 * das parcelas; no cheque especial, o limite em uso agora, senão o que o salário do ciclo passado teve de cobrir
 * (lastCycle), senão o negativo já comprometido da previsão, sem compras novas no cartão.
 */
export function debtRemaining(d: {
  kind: string
  progress: { remaining: number } | null
  limit: { inLimit: number } | null
  lastCycle?: { amount: number } | null
  forecast?: { inLimit: number; committedInLimit?: number } | null
}): number {
  if (d.kind === 'INSTALLMENTS') return d.progress?.remaining ?? 0
  if (d.limit && d.limit.inLimit > 0) return d.limit.inLimit
  if (d.lastCycle) return d.lastCycle.amount
  return d.forecast?.committedInLimit ?? d.forecast?.inLimit ?? 0
}

/** Nome da dívida para a frase: "dívida com a Mãe" quando há credor, senão o nome dela. */
export function debtLabel(d: { name: string; creditor: string }): string {
  return d.creditor.trim() ? `dívida com ${d.creditor.trim()}` : d.name
}

const GOAL_KEY = 'projection-debt-goal'
export const DEFAULT_GOAL: DebtGoal = { label: 'dívida com a mãe', amount: 1000 }

export function parseGoal(raw: string | null): DebtGoal {
  try {
    const d: unknown = raw ? JSON.parse(raw) : null
    if (typeof d !== 'object' || d === null) return DEFAULT_GOAL
    const g = d as Partial<DebtGoal>
    const ok = typeof g.label === 'string' && g.label.length <= 60 && typeof g.amount === 'number' && Number.isFinite(g.amount) && g.amount >= 0 && g.amount <= 1e9
    if (!ok) return DEFAULT_GOAL
    const goal: DebtGoal = { label: g.label as string, amount: g.amount as number }
    if (typeof g.debtId === 'string' && g.debtId.length <= 64) goal.debtId = g.debtId
    return goal
  } catch {
    return DEFAULT_GOAL
  }
}

export function loadGoal(): DebtGoal {
  try {
    return parseGoal(localStorage.getItem(GOAL_KEY))
  } catch {
    return DEFAULT_GOAL
  }
}

export function saveGoal(g: DebtGoal) {
  try {
    localStorage.setItem(GOAL_KEY, JSON.stringify(g))
  } catch {
    // sem armazenamento, a meta vale só nesta sessão
  }
}
