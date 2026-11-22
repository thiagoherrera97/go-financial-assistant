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

/** Grava a nota da transação; vazia apaga. */
export const setTransactionNote = (id: string, note: string) =>
  request<{ note: string }>(`/api/transactions/${id}/note`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ note }),
  })

/** Apaga um lançamento manual avulso (os do banco não podem ser apagados: voltariam na sincronização). */
export const deleteTransaction = (id: string) => request<void>(`/api/transactions/${id}`, { method: 'DELETE' })

