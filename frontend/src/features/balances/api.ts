import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export type Level = 'ok' | 'warning' | 'critical'

export interface BalanceAccount {
  id: string
  name: string
  last4: string
  balance: number
  /** Aplicado automaticamente pelo banco; fora do total em conta. */
  autoInvested: number | null
  /** Caixinhas da conta (dinheiro separado para render); entram no total em conta. balance é só o que está na conta. */
  reserved: { name: string; amount: number }[]
  reservedTotal: number
  /** Quanto do limite (cheque especial) a conta usa para cobrir a dívida overdraftDebt; o balance aparece zerado. */
  overdraft: number
  overdraftDebt: string
}

export interface BalanceCard {
  id: string
  name: string
  brand: string
  last4: string
  /** Valor devido informado pelo banco. */
  invoice: number
  limit: number | null
  available: number | null
  used: number | null
  usedRatio: number | null
  usageLevel: Level
  closeDate: string | null
  dueDate: string | null
  daysToDue: number | null
  dueLevel: Level
  minimumPayment: number | null
  /** Vencimento da fatura em que entram as compras feitas hoje (AAAA-MM-DD), ou null sem vencimento conhecido. */
  nextBillDue: string | null
  /** Fechamento dessa fatura; o banco raramente informa, então costuma ser estimado (closeEstimated). */
  nextClose: string | null
  closeEstimated: boolean
}

export interface BalanceInstitution {
  id: string
  name: string
  /** Cor de marca (#rrggbb) ou null. */
  color: string | null
  /** Endereço do logo na própria API, ou null (a tela usa o monograma). */
  logo: string | null
  updatedAt: string
  stale: boolean
  /** Soma das contas correntes; null se o banco só tem cartão. */
  total: number | null
  shareOfTotal: number
  accounts: BalanceAccount[]
  cards: BalanceCard[]
}

export interface Balances {
  asOf: string | null
  totalInAccount: number | null
  openInvoices: number
  institutions: BalanceInstitution[]
}

export const useBalances = () => useQuery({ queryKey: ['balances'], queryFn: () => request<Balances>('/api/balances') })


export interface SalaryPart {
  /** Dia combinado (15, 30...). */
  day: number
  /** Quando deve cair (AAAA-MM-DD): o dia útil anterior se o dia combinado não for útil. */
  expectedDate: string
  amount: number
  received: boolean
  receivedDate: string | null
  receivedAmount: number | null
}

export interface Salary {
  configured: boolean
  /** AAAA-MM */
  month: string
  parts: SalaryPart[]
  expected: number
  received: number
  pending: number
  /** Créditos de salário do mês fora das partes (13º, PLR, férias, hora extra à parte), por data. */
  extras: { date: string; amount: number }[]
  extrasTotal: number
}

export const useSalary = () => useQuery({ queryKey: ['salary'], queryFn: () => request<Salary>('/api/salary') })

export type DebtKind = 'OVERDRAFT' | 'INSTALLMENTS'

export interface DebtLine {
  label: string
  /** Entrada positiva, saída negativa. */
  amount: number
  date: string | null
  /** Gasto que ainda não aconteceu, estimado pelo mês passado. */
  estimate: boolean
}

export interface Debt {
  id: string
  kind: DebtKind
  name: string
  creditor: string
  note: string
  originalAmount: number
  /** INSTALLMENTS: dia da parcela; OVERDRAFT: dia em que a conta costuma entrar no limite. */
  dueDay: number
  accountId: string | null
  accountName: string
  freeDays: number
  installmentAmount: number
  installments: number
  /** AAAA-MM-DD */
  firstDue: string | null
  payeeMatch: string
  paidManual: number
  /** Pix e TED enviados e recebidos com quem recebe a dívida (nomes em payeeMatch), do mais recente ao mais antigo. */
  transfers: { date: string; amount: number; label: string; in: boolean }[]
  progress: {
    paid: number
    total: number
    paidAmount: number
    remaining: number
    nextDue: string | null
    overdue: boolean
    done: boolean
    payments: { date: string; amount: number; label: string; in: boolean }[]
  } | null
  limit: {
    inLimit: number
    since: string | null
    daysUsed: number
    months: { month: string; days: number; worst: number }[]
  } | null
  forecast: {
    /** Dia do mergulho (ex.: dia 10). */
    date: string
    /** Véspera do primeiro salário depois do mergulho: até onde as saídas contam. */
    until: string
    lines: DebtLine[]
    balance: number
    inLimit: number
    /** Sem as estimativas de compras novas: só o que já está lançado ou se repete todo mês. */
    committed: number
    committedInLimit: number
    exitDate: string | null
    exitLabel: string
    days: number
    withinFree: boolean
  } | null
}

export interface DebtInput {
  kind: DebtKind
  name: string
  creditor: string
  note: string
  originalAmount: number
  dueDay: number
  accountId?: string
  freeDays?: number
  installmentAmount?: number
  installments?: number
  firstDue?: string
  payeeMatch?: string
  paidManual?: number
}

/** Entrada ou saída que se repete todo mês fora o salário (aluguel, seguro, a mãe pagando uma dívida antiga...). */
export interface RecurringFlow {
  id: string
  in: boolean
  name: string
  party: string
  amount: number
  /** Quanto o valor sobe por mês (0 = fixo). */
  step: number
  /** Teto do valor (0 = sem teto). */
  maxAmount: number
  day: number
  /** AAAA-MM */
  startMonth: string
  /** Quantas vezes (0 = sem fim). */
  installments: number
  payeeMatch: string
  /** Meses em que a transação foi reconhecida. */
  done: number
  thisMonth: number
  active: boolean
  thisMonthDone: boolean
  nextDate: string | null
  nextAmount: number
  remaining: number
  ended: boolean
  matches: { date: string; amount: number; label: string; in: boolean }[]
}

export interface FlowInput {
  in: boolean
  name: string
  party: string
  amount: number
  step: number
  maxAmount: number
  day: number
  startMonth: string
  installments: number
  payeeMatch: string
}

export const useDebts = () =>
  useQuery({ queryKey: ['debts'], queryFn: () => request<{ debts: Debt[]; flows: RecurringFlow[] }>('/api/debts') })

const flowBody = (method: string, input: FlowInput) => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(input),
})
export const createFlow = (input: FlowInput) => request('/api/recurring', flowBody('POST', input))
export const updateFlow = (id: string, input: FlowInput) => request(`/api/recurring/${encodeURIComponent(id)}`, flowBody('PUT', input))
export const deleteFlow = (id: string) => request(`/api/recurring/${encodeURIComponent(id)}`, { method: 'DELETE' })

const jsonBody = (method: string, input: DebtInput) => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(input),
})

export const createDebt = (input: DebtInput) => request('/api/debts', jsonBody('POST', input))
export const updateDebt = (id: string, input: DebtInput) => request(`/api/debts/${encodeURIComponent(id)}`, jsonBody('PUT', input))
export const deleteDebt = (id: string) => request(`/api/debts/${encodeURIComponent(id)}`, { method: 'DELETE' })
