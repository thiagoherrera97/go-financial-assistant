import { describe, expect, it } from 'vitest'
import { DECISION_LABEL, canBePayment, decisionNote, formatDay, totals, transferSummary, truncatedNote } from './transfers'
import type { TransferItem } from '../api'

const t = (over: Partial<TransferItem> = {}): TransferItem => ({
  id: 'x',
  date: '2026-11-03',
  amount: 250.5,
  direction: 'out',
  account: 'Itaú 1234',
  description: 'Pix enviado',
  category: 'OTHER',
  ...over,
})

describe('formatDay', () => {
  it('troca a ordem sem passar por Date (o dia não muda com o fuso)', () => {
    expect(formatDay('2026-11-03')).toBe('03/11/2026')
    expect(formatDay('2026-12-01')).toBe('01/12/2026')
    expect(formatDay('lixo')).toBe('lixo')
  })
})

describe('transferSummary', () => {
  it('mostra sentido, valor, data e conta', () => {
    expect(transferSummary(t())).toMatch(/^Saída · R\$\s250,50 · 03\/11\/2026 · Itaú 1234$/)
    expect(transferSummary(t({ direction: 'in', account: '' }))).toMatch(/^Entrada · R\$\s250,50 · 03\/11\/2026$/)
  })
})

describe('canBePayment', () => {
  it('só saídas podem ser "Pagamento"', () => {
    expect(canBePayment(t())).toBe(true)
    expect(canBePayment(t({ direction: 'in' }))).toBe(false)
  })
})

describe('decisionNote e rótulos', () => {
  const label = (v: string) => (v === 'HOUSING' ? 'Moradia' : v)
  it('explica o efeito de cada decisão', () => {
    expect(decisionNote(t({ decision: 'OWN' }), label)).toContain('Não conta mais')
    expect(decisionNote(t({ decision: 'PERSON', category: 'PEOPLE' }), label)).toBe('Categoria: Pessoas.')
    expect(decisionNote(t({ decision: 'PAYMENT', category: 'HOUSING' }), label)).toBe('Categoria: Moradia.')
    expect(decisionNote(t(), label)).toBe('')
  })
  it('tem as 3 escolhas com os nomes da tela', () => {
    expect(Object.values(DECISION_LABEL)).toEqual(['Entre contas minhas', 'Para uma pessoa', 'Pagamento'])
  })
})

describe('truncatedNote e totals', () => {
  it('avisa só quando a lista veio cortada', () => {
    expect(truncatedNote(300, 300)).toBe('')
    expect(truncatedNote(300, 412)).toBe('Mostrando as 300 mais recentes de 412.')
  })
  it('soma entradas e saídas separadas', () => {
    expect(totals([t({ amount: 100 }), t({ amount: 50 }), t({ direction: 'in', amount: 30 })])).toEqual({ out: 150, in: 30 })
    expect(totals([])).toEqual({ out: 0, in: 0 })
  })
})
