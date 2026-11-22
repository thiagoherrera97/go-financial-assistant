import { describe, expect, it } from 'vitest'
import { formatBRL } from '../../../shared/lib/format'
import { cardNotes, formatCycle, leftoverLabel, sheetLeftover } from './month'

describe('tela do mês', () => {
  const card = { name: 'SANTANDER', total: 100, categories: [], forecast: 0, adjustment: 0, refunds: 0 }

  it('mostra só as linhas extras que existem', () => {
    expect(cardNotes(card)).toEqual([])
    const notes = cardNotes({ ...card, forecast: 669, adjustment: -69 })
    expect(notes.map((n) => [n.label, n.amount])).toEqual([
      ['Parcelas previstas', 669],
      ['Créditos não detalhados', -69],
    ])
    expect(cardNotes({ ...card, adjustment: 279.89 })[0].label).toBe('Não detalhado pelo banco')
  })

  it('sobra ou faltou', () => {
    expect(leftoverLabel(10)).toBe('Sobra')
    expect(leftoverLabel(-1)).toBe('Faltou')
  })
})

describe('ciclo do salário', () => {
  it('mostra o período do ciclo', () => {
    expect(formatCycle('2026-11-13', '2026-12-14')).toBe('13/nov – 14/dez')
  })

  const base = { leftover: 100, leftoverWithPending: 400, pendingTotal: 300, carryover: null, leftoverWithCarryover: 400 }
  it('a sobra soma o que veio do ciclo anterior', () => {
    expect(sheetLeftover({ ...base, carryover: 731.27, leftoverWithCarryover: 1131.27 })).toEqual({
      value: 1131.27,
      note: `contando ${formatBRL(731.27)} do ciclo anterior e o que falta receber`,
    })
  })
  it('sem ciclo anterior, fica como antes', () => {
    expect(sheetLeftover(base).value).toBe(400)
    expect(sheetLeftover({ ...base, pendingTotal: 0, leftoverWithPending: 100, leftoverWithCarryover: 100 })).toEqual({ value: 100, note: 'entradas − despesas' })
  })
})
