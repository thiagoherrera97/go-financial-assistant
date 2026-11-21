import { describe, expect, it } from 'vitest'
import { emptyFlowForm, flowRule, toFlowInput } from './flows'

describe('toFlowInput', () => {
  it('monta a ajuda que cresce até um teto', () => {
    const f = { ...emptyFlowForm(true, '2026-11'), name: 'Desmame do aluguel', party: 'Mãe', amount: '400', step: '100', maxAmount: '1.300,00' }
    expect(toFlowInput(f)).toEqual({
      input: {
        in: true, name: 'Desmame do aluguel', party: 'Mãe', amount: 400, step: 100, maxAmount: 1300, day: 10, startMonth: '2026-11',
        installments: 0, payeeMatch: '',
      },
    })
  })
  it('recusa teto menor que o valor', () => {
    expect(toFlowInput({ ...emptyFlowForm(true, '2026-11'), name: 'x', amount: '500', maxAmount: '100' })).toEqual({
      error: 'O teto não pode ser menor que o valor',
    })
  })
})

describe('flowRule', () => {
  it('descreve o combinado', () => {
    const money = (v: number) => `R$ ${v}`
    expect(flowRule({ day: 10, step: 100, maxAmount: 1300, installments: 0 }, money)).toBe('todo dia 10 · sobe R$ 100 por mês até R$ 1300')
    expect(flowRule({ day: 10, step: 0, maxAmount: 0, installments: 10 }, money)).toBe('todo dia 10 · 10x')
  })
})
