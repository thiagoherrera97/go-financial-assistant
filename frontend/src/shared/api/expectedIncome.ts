import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { request } from './request'

export interface ExpectedIncome {
  source: 'SALARY' | 'RECURRING'
  label: string
  /** AAAA-MM-DD em que deve cair. */
  date: string
  amount: number
  /** A data passou e o crédito ainda não chegou do banco. */
  late: boolean
}

/** Receitas do ciclo que já se sabe que vão cair e ainda não chegaram (somem quando o crédito real sincroniza). */
export const useExpectedIncome = (month: string | null, enabled: boolean) =>
  useQuery({
    queryKey: ['expected-income', month],
    queryFn: () => request<{ items: ExpectedIncome[]; total: number }>(`/api/transactions/expected-income?month=${month}`),
    enabled: enabled && !!month,
    placeholderData: keepPreviousData,
  })
