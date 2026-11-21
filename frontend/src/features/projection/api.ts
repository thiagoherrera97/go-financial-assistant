import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface Projection {
  assumptions: {
    income: number
    /** Fontes de renda recorrentes e quanto cada uma rende em um mês comum. */
    incomeSources: { label: string; monthly: number }[]
    fixed: number
    variable: number
    basedOn: number
  }
  months: { month: string; fixed: number; installment: number; variable: number }[]
}

export const useProjection = (months: number) =>
  useQuery({
    queryKey: ['projection', months],
    queryFn: () => request<Projection>(`/api/projection?months=${months}`),
    placeholderData: keepPreviousData,
  })

/** Dívida pessoal (GET /api/debts), só com o que a projeção usa. */
export interface Debt {
  id: string
  kind: 'OVERDRAFT' | 'INSTALLMENTS'
  name: string
  creditor: string
  progress: { remaining: number; done: boolean } | null
  limit: { inLimit: number } | null
  /** Cheque especial: previsão do próximo mergulho no limite. */
  forecast: { inLimit: number } | null
}

export const useDebts = () =>
  useQuery({
    queryKey: ['debts'],
    queryFn: () => request<{ debts: Debt[] }>('/api/debts').then((r) => r.debts ?? []),
  })

/** O que o card de quanto ainda dá para gastar usa da tela do mês (GET /api/month). */
export interface MonthTotals {
  leftoverWithPending: number
  fixedTotal: number
}

export const useMonthTotals = (month: string) =>
  useQuery({
    queryKey: ['month-sheet', month],
    queryFn: () => request<MonthTotals>(`/api/month?month=${month}`),
  })
