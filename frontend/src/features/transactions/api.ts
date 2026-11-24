import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface Transaction {
  id: string
  date: string
  description: string
  category: string
  categoryLabel: string
  /** Código da subcategoria; ausente quando não tem (receitas, Pessoas, Outros). */
  subcategory?: string
  subcategoryLabel?: string
  /** true = essencial, false = supérfluo, null = a revisar (sempre null fora das despesas). */
  essential: boolean | null
  paymentMethod: string
  paymentMethodLabel: string
  kind: 'EXPENSE' | 'INCOME' | 'TRANSFER'
  transferDirection?: 'IN' | 'OUT'
  type: string
  status: 'PENDING' | 'PAID'
  amount: number
  installmentNumber?: number
  accountId?: string
  accountName?: string
  source: 'MANUAL' | 'OPEN_FINANCE'
  /** Nota escrita por você ("o que foi"); vazia sem nota. */
  note: string
  /** Instante (ISO, com hora) em que o banco registrou; ausente em manuais, parcelas e bancos que só mandam o dia. */
  bankTime?: string
  /** Dia da compra (AAAA-MM-DD) numa parcela 2+ do cartão, cuja `date` é a da fatura; ausente nos demais. */
  purchaseDate?: string
}

export interface TransactionGroup {
  /** Categoria (enum) ou data AAAA-MM-DD. */
  key: string
  label: string
  count: number
  expense: number
  income: number
  /** Aplicações + resgates (volume movimentado, não é dinheiro novo). */
  transfer: number
  /** Saiu para o investimento ou a caixinha. */
  applied: number
  /** Voltou do investimento ou da caixinha. */
  redeemed: number
}

export interface TransactionPage {
  items: Transaction[]
  total: number
  page: number
  limit: number
}

/** query já vem montada e codificada por quem chama (URLSearchParams). */
export const useTransactions = (query: string) =>
  useQuery({
    queryKey: ['transactions', query],
    queryFn: () => request<TransactionPage>(`/api/transactions?${query}`),
    placeholderData: keepPreviousData,
  })

/** query já vem montada e codificada (URLSearchParams); by: "category" ou "day". */
export const useTransactionGroups = (query: string, by: 'category' | 'day') =>
  useQuery({
    queryKey: ['transaction-groups', by, query],
    queryFn: () => request<TransactionGroup[]>(`/api/transactions/groups?${query}&by=${by}`),
    placeholderData: keepPreviousData,
  })

/** Apaga um lançamento manual avulso (os do banco não podem ser apagados: voltariam na sincronização). */
export const deleteTransaction = (id: string) => request<void>(`/api/transactions/${id}`, { method: 'DELETE' })

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

/** As recorrências vêm junto com as dívidas em /api/debts (a previsão do cheque especial usa as duas). */
export const useRecurringFlows = () =>
  useQuery({
    queryKey: ['recurring-flows'],
    queryFn: () => request<{ flows: RecurringFlow[] }>('/api/debts'),
    select: (data) => data.flows,
  })

const flowBody = (method: string, input: FlowInput) => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(input),
})
export const createFlow = (input: FlowInput) => request('/api/recurring', flowBody('POST', input))
export const updateFlow = (id: string, input: FlowInput) => request(`/api/recurring/${encodeURIComponent(id)}`, flowBody('PUT', input))
export const deleteFlow = (id: string) => request(`/api/recurring/${encodeURIComponent(id)}`, { method: 'DELETE' })
