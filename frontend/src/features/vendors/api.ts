import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface VendorPayment {
  id: string
  amount: number
  /** AAAA-MM-DD */
  paidOn: string
  /** Transação do extrato ligada; nulo no pagamento lançado à mão. */
  transactionId: string | null
  description: string
}

export interface Vendor {
  id: string
  name: string
  contracted: number
  /** Último dia para quitar (AAAA-MM-DD); nulo = sem prazo. */
  dueDate: string | null
  paid: number
  remaining: number
  done: boolean
  /** Aportes do próximo até o vencimento. */
  contributions: number
  /** Vence antes do próximo aporte (ou já venceu) e ainda falta pagar. */
  late: boolean
  payments: VendorPayment[]
}

export interface VendorPlan {
  name: string
  contributionDay: number
  /** Nome da caixinha ligada; vazio = valor informado à mão. */
  savingsBox: string
  savedManual: number
}

export interface VendorSummary {
  plan: VendorPlan
  /** Caixinhas das contas correntes que dá para ligar. */
  boxes: string[]
  contracted: number
  paid: number
  saved: number
  savedFromBox: boolean
  remaining: number
  nextContribution: string
  lastDue: string | null
  monthsLeft: number
  /** Quanto guardar por aporte para cumprir todos os prazos. */
  perMonth: number
  /** A conta da planilha: o que falta ÷ aportes até o último vencimento. */
  flatPerMonth: number
  /** Fornecedor cujo prazo pede mais que a média; nulo quando a média basta. */
  tight: string | null
  surplus: number | null
  fits: boolean | null
  vendors: Vendor[]
}

export interface VendorInput {
  name: string
  contracted: number
  dueDate: string
  /** Só na criação: o que já foi pago antes. */
  paid?: number
}

/** Uma saída do extrato candidata a pagamento de fornecedor. */
export interface TransactionHit {
  id: string
  date: string
  description: string
  note: string
  amount: number
  kind: string
  transferDirection?: string
  status: string
  accountName?: string
}

const json = (method: string, data: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(data),
})

export const VENDORS_KEY = ['vendors']

export const useVendors = () => useQuery({ queryKey: VENDORS_KEY, queryFn: () => request<VendorSummary>('/api/vendors') })

export const savePlan = (p: VendorPlan) => request('/api/vendors/plan', json('PUT', p))
export const createVendor = (v: VendorInput) => request('/api/vendors', json('POST', v))
export const updateVendor = (id: string, v: VendorInput) => request(`/api/vendors/${encodeURIComponent(id)}`, json('PUT', v))
export const deleteVendor = (id: string) => request(`/api/vendors/${encodeURIComponent(id)}`, { method: 'DELETE' })

export const addPayment = (vendorId: string, p: { amount: number; paidOn: string } | { transactionId: string }) =>
  request(`/api/vendors/${encodeURIComponent(vendorId)}/payments`, json('POST', p))

export const deletePayment = (vendorId: string, paymentId: string) =>
  request(`/api/vendors/${encodeURIComponent(vendorId)}/payments/${encodeURIComponent(paymentId)}`, { method: 'DELETE' })

/** Busca no extrato pela descrição ou nota (as mais recentes primeiro). */
export const searchTransactions = (q: string) =>
  request<{ items: TransactionHit[] }>(`/api/transactions?q=${encodeURIComponent(q)}&limit=20`).then((r) => r.items)
