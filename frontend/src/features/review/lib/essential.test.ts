import { describe, expect, it } from 'vitest'
import { essentialQuery } from './essential'

describe('essentialQuery', () => {
  it('pede as despesas do mês da aba, do maior valor para o menor', () => {
    const q = new URLSearchParams(essentialQuery('2026-11', 'false', 20))
    expect(Object.fromEntries(q)).toEqual({ month: '2026-11', kind: 'EXPENSE', essential: 'false', sort: 'amount', limit: '20' })
  })
})
