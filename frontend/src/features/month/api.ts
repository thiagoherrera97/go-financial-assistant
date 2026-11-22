import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface SheetLine {
  label: string
  /** AAAA-MM-DD */
  date: string
  amount: number
  /** Nota escrita por você; vazia sem nota. */
  note: string
}

export interface SheetItem {
  label: string
  amount: number
  count: number
  /** Tem lançamento pendente. */
  pending: boolean
  /** Os lançamentos que a linha soma. */
  lines: SheetLine[]
}

export interface SheetCard {
  name: string
  total: number
  /** Compras reais menos estornos, por categoria. */
  categories: SheetItem[]
  /** Parcelas previstas (o banco ainda não mandou). */
  forecast: number
  /** Ajuste pelo total da fatura do banco (pode ser negativo). */
  adjustment: number
  /** Estornos já abatidos nas categorias (negativo). */
  refunds: number
}

export interface MonthSheet {
  month: string
  income: SheetItem[]
  incomeTotal: number
  pending: { label: string; date: string; amount: number }[]
  pendingTotal: number
  /** Saídas recorrentes (aluguel, seguro, financiamento) que ainda não saíram; a sobra já as desconta. */
  toPay: { label: string; date: string; amount: number }[]
  toPayTotal: number
  fixed: SheetItem[]
  fixedTotal: number
  other: SheetItem[]
  otherTotal: number
  cards: SheetCard[]
  cardsTotal: number
  expensesTotal: number
  leftover: number
  leftoverWithPending: number
  /** Aplicado − resgatado no mês; fora da sobra. */
  invested: number
  /** Primeiro e último dia do ciclo do salário (AAAA-MM-DD). */
  startsOn: string
  endsOn: string
  /** Sobra do ciclo anterior; null quando ele não tem lançamentos. */
  carryover: number | null
  leftoverWithCarryover: number
}

export const useMonthSheet = (month: string) =>
  useQuery({
    queryKey: ['month-sheet', month],
    queryFn: () => request<MonthSheet>(`/api/month?month=${month}`),
    placeholderData: keepPreviousData,
  })
