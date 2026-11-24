import { describe, expect, it } from 'vitest'
import { decidedNote, decisionOf, initialChoice, salaryOptions } from './salary'
import type { SalaryReviewItem } from '../api'

const parts = [
  { day: 15, amount: 5200 },
  { day: 30, amount: 4366.62 },
]
const item = (over: Partial<SalaryReviewItem> = {}): SalaryReviewItem => ({
  id: 'x', date: '2026-12-18', amount: 9566.62, label: 'SALARIO', suggest: 'MONTH', month: '2026-12', ...over,
})

describe('salaryOptions', () => {
  it('oferece o mês do palpite e o seguinte (salário adiantado), cada parte e os extras', () => {
    const values = salaryOptions(item(), parts).map((o) => o.value)
    expect(values).toEqual(['MONTH:2026-12', 'MONTH:2027-01', 'PART:15', 'PART:30', 'OVERTIME', 'PLR', 'THIRTEENTH', 'VACATION', 'OTHER'])
  })
  it('dá nome legível a cada opção', () => {
    const labels = salaryOptions(item(), parts).map((o) => o.label)
    expect(labels[0]).toBe('Salário de dezembro inteiro')
    expect(labels[1]).toBe('Salário de janeiro inteiro (adiantado)')
    expect(labels[2]).toMatch(/^Parte do dia 15 \(R\$\s5\.200,00\)$/)
    expect(labels).toContain('PLR')
  })
})

describe('initialChoice', () => {
  it('parte do palpite do app', () => {
    expect(initialChoice(item())).toBe('MONTH:2026-12')
    expect(initialChoice(item({ suggest: 'PART', day: 30 }))).toBe('PART:30')
  })
  it('extra começa sem escolha: o app não sabe se é PLR ou hora extra', () => {
    expect(initialChoice(item({ suggest: '' }))).toBe('')
  })
})

describe('decisionOf', () => {
  it('monta o corpo da decisão', () => {
    expect(decisionOf('MONTH:2027-01')).toEqual({ kind: 'MONTH', month: '2027-01' })
    expect(decisionOf('PART:30', '2026-12')).toEqual({ kind: 'PART', month: '2026-12', day: 30 })
    expect(decisionOf('PLR')).toEqual({ kind: 'PLR' })
  })
})

describe('decidedNote', () => {
  it('descreve a decisão', () => {
    expect(decidedNote({ kind: 'MONTH', kindLabel: 'Salário do mês inteiro', month: '2027-01' })).toBe('Salário de janeiro inteiro')
    expect(decidedNote({ kind: 'PART', kindLabel: 'Parte do salário', month: '2026-12', day: 30 })).toBe('Parte do dia 30 de dezembro')
    expect(decidedNote({ kind: 'PLR', kindLabel: 'PLR' })).toBe('PLR')
  })
})
