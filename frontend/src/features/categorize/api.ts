import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface UncategorizedGroup {
  /** Descrição normalizada: identifica a conta. */
  key: string
  label: string
  count: number
  total: number
  /** Último lançamento (AAAA-MM). */
  last: string
  /** Pix, TED ou transferência: só a pessoa sabe do que se trata, e nunca vai à IA. */
  transfer: boolean
}

export interface Categorize {
  /** Categorias atribuíveis; OTHER significa "manter em Outros". */
  categories: { value: string; label: string }[]
  groups: UncategorizedGroup[]
  ai: { enabled: boolean; blockedReason: string; provider: string; hash: string; bytes: number; context: { contas: { id: string; nome: string; lancamentos: number; total_gasto: number; ultimo_mes: string }[] } }
}

export const useCategorize = () => useQuery({ queryKey: ['categorize'], queryFn: () => request<Categorize>('/api/categorize') })

/** Classifica uma conta: reclassifica as despesas dela em Outros e vale nas próximas sincronizações. */
export const setCategoryRule = (key: string, category: string) =>
  request<{ changed: number }>('/api/categorize/rules', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, category }),
  })

/** Pede à IA categorias para as contas da prévia (o hash garante que é o que a pessoa viu). Nada é gravado. */
export const suggestCategories = (hash: string) =>
  request<{ suggestions: { key: string; category: string }[] }>('/api/categorize/suggest', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ hash }),
  })

export type TransferDecision = 'OWN' | 'PERSON' | 'PAYMENT'

export interface TransferItem {
  id: string
  /** AAAA-MM-DD */
  date: string
  amount: number
  /** "out" é saída, "in" é entrada. */
  direction: 'out' | 'in'
  account: string
  description: string
  category: string
  /** Só nas decididas. */
  decision?: TransferDecision
  decidedAt?: string
}

export interface TransfersReview {
  pending: TransferItem[]
  /** Quantas esperam decisão no total (a lista pode vir cortada). */
  pendingTotal: number
  decided: TransferItem[]
  /** Categorias que "Pagamento" aceita. */
  categories: { value: string; label: string }[]
}

export const useTransfersReview = () => useQuery({ queryKey: ['transfers-review'], queryFn: () => request<TransfersReview>('/api/transfers/review') })

/** Decide uma transferência. Nada é decidido sem a escolha da pessoa; category só vale em PAYMENT. */
export const decideTransfer = (id: string, decision: TransferDecision, category?: string) =>
  request<{ ok: boolean }>(`/api/transfers/review/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(decision === 'PAYMENT' ? { decision, category } : { decision }),
  })

/** Desfaz a decisão: o status e a categoria de antes voltam. */
export const undoTransfer = (id: string) => request<{ ok: boolean }>(`/api/transfers/review/${encodeURIComponent(id)}`, { method: 'DELETE' })

export type SalaryKind = 'MONTH' | 'PART' | 'OVERTIME' | 'PLR' | 'THIRTEENTH' | 'VACATION' | 'OTHER'

/** Crédito de salário que o app não reconheceu com certeza (valor fora do combinado, ou extra). */
export interface SalaryReviewItem {
  id: string
  date: string
  amount: number
  label: string
  /** Palpite do app: MONTH (salário do mês inteiro), PART (uma parte, ver day) ou vazio (extra: o app não sabe qual). */
  suggest: '' | 'MONTH' | 'PART'
  /** AAAA-MM: o ciclo do palpite. */
  month: string
  day?: number
}

export interface SalaryDecidedItem {
  id: string
  date: string
  amount: number
  label: string
  kind: SalaryKind
  kindLabel: string
  month?: string
  day?: number
}

export interface SalaryReview {
  pending: SalaryReviewItem[]
  decided: SalaryDecidedItem[]
  /** Partes do salário configuradas; vazio sem salário em Configurações → Renda. */
  parts: { day: number; amount: number }[]
  kinds: { value: SalaryKind; label: string }[]
}

export interface SalaryDecision {
  kind: SalaryKind
  month?: string
  day?: number
}

export const useSalaryReview = () => useQuery({ queryKey: ['salary-review'], queryFn: () => request<SalaryReview>('/api/salary/review') })

export const decideSalary = (id: string, decision: SalaryDecision) =>
  request<void>(`/api/salary/review/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(decision),
  })

export const undoSalary = (id: string) => request<void>(`/api/salary/review/${encodeURIComponent(id)}`, { method: 'DELETE' })
