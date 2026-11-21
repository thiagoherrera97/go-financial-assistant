// "Quanto ainda dá para gastar neste mês": parte do que a tela do mês já mostra (entradas, salário a cair e despesas
// lançadas) e tira as contas fixas que ainda não caíram, estimadas pela média de fixas da projeção.

export interface MonthNumbers {
  /** Entradas menos despesas já lançadas, contando o salário que ainda vai cair. */
  leftoverWithPending: number
  /** Fixas e parcelas fora do cartão já lançadas no mês. */
  fixedTotal: number
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

/** Dias que faltam no mês de `today`, contando hoje. */
export function daysLeftInMonth(today: Date): number {
  const last = new Date(today.getFullYear(), today.getMonth() + 1, 0).getDate()
  return last - today.getDate() + 1
}

export function spendable(month: MonthNumbers, averageFixed: number, goal: number, today: Date): Spendable {
  const fixedToCome = Math.max(0, averageFixed - month.fixedTotal)
  const toBreakEven = month.leftoverWithPending - fixedToCome
  const toReachGoal = toBreakEven - Math.max(0, goal)
  const daysLeft = daysLeftInMonth(today)
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
}

const GOAL_KEY = 'projection-debt-goal'
export const DEFAULT_GOAL: DebtGoal = { label: 'dívida com a mãe', amount: 1000 }

export function parseGoal(raw: string | null): DebtGoal {
  try {
    const d: unknown = raw ? JSON.parse(raw) : null
    if (typeof d !== 'object' || d === null) return DEFAULT_GOAL
    const g = d as Partial<DebtGoal>
    const ok = typeof g.label === 'string' && g.label.length <= 60 && typeof g.amount === 'number' && Number.isFinite(g.amount) && g.amount >= 0 && g.amount <= 1e9
    return ok ? { label: g.label as string, amount: g.amount as number } : DEFAULT_GOAL
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
