import { describe, expect, it } from 'vitest'
import { capiMood } from './capiMood'
import type { BalanceCard, Balances } from '../api'

const card = (over: Partial<BalanceCard> = {}): BalanceCard => ({
  id: 'c', name: 'Visa', brand: 'VISA', last4: '1234', invoice: 500, limit: 5000, available: 4500, used: 500, usedRatio: 0.1,
  usageLevel: 'ok', closeDate: null, dueDate: '2026-12-10', daysToDue: 10, dueLevel: 'ok', minimumPayment: null,
  nextBillDue: null, nextClose: null, closeEstimated: false, ...over,
})

const data = (total: number | null, cards: BalanceCard[] = []): Balances => ({
  asOf: null, totalInAccount: total, openInvoices: 0,
  institutions: [{ id: 'b', name: 'Banco', color: null, logo: null, updatedAt: '', stale: false, total, shareOfTotal: 1, accounts: [], cards }],
})

describe('capiMood', () => {
  it('fica feliz com saldo positivo e nada apertando', () => {
    expect(capiMood(data(300, [card()]), false).mood).toBe('happy')
  })

  it('fica preocupada com a conta no vermelho', () => {
    expect(capiMood(data(-10), false).mood).toBe('worried')
  })

  it('fica preocupada com fatura crítica e diz quando vence', () => {
    const s = capiMood(data(300, [card({ name: 'Santander', dueLevel: 'critical', daysToDue: 1 })]), false)
    expect(s).toEqual({ mood: 'worried', line: 'A fatura do Santander vence amanhã.' })
  })

  it('ignora vencimento crítico de fatura zerada', () => {
    expect(capiMood(data(300, [card({ invoice: 0, dueLevel: 'critical' })]), false).mood).toBe('happy')
  })

  it('fica preocupada com limite crítico', () => {
    expect(capiMood(data(300, [card({ usageLevel: 'critical' })]), false).mood).toBe('worried')
  })

  it('fica atenta (neutra) com cartão em atenção', () => {
    expect(capiMood(data(300, [card({ usageLevel: 'warning' })]), false).mood).toBe('neutral')
  })

  it('com valores ocultos fica neutra para não entregar o saldo', () => {
    expect(capiMood(data(-10), true).mood).toBe('neutral')
  })
})
