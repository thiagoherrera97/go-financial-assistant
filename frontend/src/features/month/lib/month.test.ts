import { describe, expect, it } from 'vitest'
import { cardNotes, leftoverLabel } from './month'

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
