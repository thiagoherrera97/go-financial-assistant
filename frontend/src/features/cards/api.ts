import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export type BillState = 'CLOSED' | 'OPEN' | 'FUTURE'
export type BillOrigin = 'BANK' | 'MANUAL' | 'FORECAST' | 'GAP' | 'REPLACED'

export interface CardBill {
  /** Vencimento (AAAA-MM-DD). */
  due: string
  state: BillState
  /** Total da fatura fechada informado pelo banco; null enquanto não fechou. */
  bankTotal: number | null
  /** Lançamentos do banco, manuais e parcelas trocadas. */
  detailed: number
  /** Parcelas previstas pelo app (o banco ainda não mandou). */
  forecast: number
  /** Ajuste "não detalhado no Open Finance": o que o banco cobrou e o app não conhece. */
  gap: number
  /** detailed + forecast: o que o app conhece. */
  known: number
  /** bankTotal − known (0 dentro da tolerância de centavos ou sem total do banco). */
  difference: number
  total: number
  count: number
}

export interface CardWithBills {
  id: string
  name: string
  brand: string
  last4: string
  /** Saldo devedor que o banco informa agora (inclui parcelas futuras). */
  bankBalance: number
  bills: CardBill[]
}

export interface BillLine {
  id: string
  date: string
  /** Dia da compra numa parcela 2+. */
  purchaseDate?: string
  description: string
  category: string
  categoryLabel: string
  amount: number
  installmentNumber?: number
  origin: BillOrigin
  pending: boolean
  note: string
}

export const useCardBills = () =>
  useQuery({ queryKey: ['card-bills'], queryFn: () => request<{ cards: CardWithBills[] }>('/api/card-bills') })

export const useBillLines = (card: string | null, due: string | null) =>
  useQuery({
    queryKey: ['card-bill-lines', card, due],
    queryFn: () => request<{ lines: BillLine[] }>(`/api/card-bills/${card}/${due}`),
    enabled: card !== null && due !== null,
  })
