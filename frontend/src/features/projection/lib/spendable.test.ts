import { describe, expect, it } from 'vitest'
import { DEFAULT_GOAL, daysLeftInMonth, debtLabel, debtRemaining, parseGoal, spendable } from './spendable'

describe('spendable', () => {
  const today = new Date(2026, 10, 21) // 21/11: faltam 10 dias contando hoje

  it('tira as fixas que ainda não caíram e a meta', () => {
    const s = spendable({ leftoverWithPending: 3000, fixedTotal: 1200 }, 2000, 1000, today)
    expect(s.fixedToCome).toBe(800)
    expect(s.toBreakEven).toBe(2200)
    expect(s.toReachGoal).toBe(1200)
    expect(s.daysLeft).toBe(10)
    expect(s.perDayBreakEven).toBe(220)
    expect(s.perDayGoal).toBe(120)
  })

  it('não conta fixa negativa quando já caiu mais que a média', () => {
    const s = spendable({ leftoverWithPending: 500, fixedTotal: 2500 }, 2000, 0, today)
    expect(s.fixedToCome).toBe(0)
    expect(s.toBreakEven).toBe(500)
  })

  it('mostra quanto falta quando já não fecha', () => {
    const s = spendable({ leftoverWithPending: 300, fixedTotal: 0 }, 1000, 1000, today)
    expect(s.toBreakEven).toBe(-700)
    expect(s.toReachGoal).toBe(-1700)
    expect(s.perDayBreakEven).toBe(0)
    expect(s.perDayGoal).toBe(0)
  })

  it('conta os dias até o fim do mês', () => {
    expect(daysLeftInMonth(new Date(2026, 1, 28))).toBe(1)
    expect(daysLeftInMonth(new Date(2026, 0, 1))).toBe(31)
  })
})

describe('parseGoal', () => {
  it('lê a meta salva e descarta lixo', () => {
    expect(parseGoal(JSON.stringify({ label: 'carro', amount: 500 }))).toEqual({ label: 'carro', amount: 500 })
    expect(parseGoal('{"label":1}')).toEqual(DEFAULT_GOAL)
    expect(parseGoal('nada')).toEqual(DEFAULT_GOAL)
    expect(parseGoal(null)).toEqual(DEFAULT_GOAL)
    expect(parseGoal(JSON.stringify({ label: 'x', amount: 1, debtId: 'abc' }))).toEqual({ label: 'x', amount: 1, debtId: 'abc' })
  })
})

describe('debtRemaining', () => {
  it('usa o que resta das parcelas ou o limite em uso', () => {
    expect(debtRemaining({ kind: 'INSTALLMENTS', progress: { remaining: 4500 }, limit: null })).toBe(4500)
    expect(debtRemaining({ kind: 'OVERDRAFT', progress: null, limit: { inLimit: 2300 } })).toBe(2300)
    expect(debtRemaining({ kind: 'OVERDRAFT', progress: null, limit: null })).toBe(0)
    expect(debtRemaining({ kind: 'OVERDRAFT', progress: null, limit: { inLimit: 0 }, forecast: { inLimit: 3434.83 } })).toBe(3434.83)
  })

  it('nomeia pela pessoa quando há credor', () => {
    expect(debtLabel({ name: 'Carro', creditor: 'Mãe' })).toBe('dívida com Mãe')
    expect(debtLabel({ name: 'Carro', creditor: ' ' })).toBe('Carro')
  })
})
