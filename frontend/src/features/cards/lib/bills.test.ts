import { describe, expect, it } from 'vitest'
import { billCheck, billMonth, pickBill } from './bills'
import type { CardBill } from '../api'

const bill = (due: string, over: Partial<CardBill> = {}): CardBill => ({
  due, state: 'CLOSED', bankTotal: 100, detailed: 100, forecast: 0, gap: 0, known: 100, difference: 0, total: 100, count: 3, ...over,
})

describe('faturas do cartão', () => {
  it('abre a fatura pedida, senão a aberta, senão a última fechada', () => {
    const bills = [bill('2026-10-10'), bill('2026-11-10'), bill('2026-12-10', { state: 'OPEN', bankTotal: null }), bill('2027-01-10', { state: 'FUTURE', bankTotal: null })]
    expect(pickBill(bills, '2026-10-10')).toBe(0)
    expect(pickBill(bills, '2020-01-01')).toBe(2)
    expect(pickBill(bills, null)).toBe(2)
    expect(pickBill(bills.slice(0, 2), null)).toBe(1)
    expect(pickBill([bill('2027-01-10', { state: 'FUTURE', bankTotal: null })], null)).toBe(0)
    expect(pickBill([], null)).toBe(-1)
  })

  it('mês da fatura', () => {
    expect(billMonth('2026-12-10')).toBe('dez/26')
  })

  it('confere com o banco', () => {
    expect(billCheck(bill('2026-11-10')).tone).toBe('good')
    const more = billCheck(bill('2026-11-10', { bankTotal: 150, known: 130, difference: 20 }))
    expect(more.tone).toBe('bad')
    expect(more.title).toContain('a mais')
    expect(billCheck(bill('2026-11-10', { bankTotal: 100, known: 130, difference: -30 })).title).toContain('passam')
    expect(billCheck(bill('2026-12-10', { state: 'OPEN', bankTotal: null })).tone).toBe('neutral')
    expect(billCheck(bill('2026-10-10', { bankTotal: 3054.94, known: 216.7, difference: 2838.24 }), true).tone).toBe('neutral')
  })
})
