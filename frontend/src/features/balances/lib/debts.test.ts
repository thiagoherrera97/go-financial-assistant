import { describe, expect, it } from 'vitest'
import { debtOutstanding, emptyDebtForm, nextDay, nextDueDate, salaryFallsShort, toDebtInput } from './debts'
import type { Debt } from '../api'

describe('toDebtInput', () => {
  it('monta a dívida parcelada com o total das parcelas', () => {
    const f = { ...emptyDebtForm('INSTALLMENTS', '2026-12-10'), name: 'Mac mini', creditor: 'Mãe', installmentAmount: '320', payeeMatch: ' Maria ' }
    const r = toDebtInput(f)
    expect(r).toEqual({
      input: {
        kind: 'INSTALLMENTS', name: 'Mac mini', creditor: 'Mãe', note: '', dueDay: 10, originalAmount: 3200, installmentAmount: 320,
        installments: 10, firstDue: '2026-12-10', payeeMatch: 'Maria', paidManual: 0,
      },
    })
  })

  it('exige a conta na dívida coberta pelo limite', () => {
    const f = { ...emptyDebtForm('OVERDRAFT', ''), name: 'Ajuda do carro', amount: '6.000,00' }
    expect(toDebtInput(f)).toEqual({ error: 'Escolha a conta cujo limite cobre a dívida' })
    const ok = toDebtInput({ ...f, accountId: 'acc' })
    expect('input' in ok && ok.input.originalAmount).toBe(6000)
  })
})

describe('debtOutstanding', () => {
  const base = { progress: null, limit: null, forecast: null, lastCycle: null } as unknown as Debt
  it('usa o que falta, o limite em uso ou a previsão', () => {
    expect(debtOutstanding({ ...base, progress: { remaining: 2880 } } as Debt)).toBe(2880)
    expect(debtOutstanding({ ...base, limit: { inLimit: 900 }, forecast: { inLimit: 1200 } } as Debt)).toBe(900)
    expect(debtOutstanding({ ...base, limit: { inLimit: 0 }, forecast: { inLimit: 1200, committedInLimit: 800 } } as Debt)).toBe(800)
    expect(debtOutstanding({ ...base, limit: { inLimit: 0 }, lastCycle: { amount: 4877.11 } } as Debt)).toBe(4877.11)
  })
})

describe('nextDueDate', () => {
  it('pula para o mês seguinte depois do dia', () => {
    expect(nextDueDate(new Date(2026, 10, 21), 10)).toBe('2026-12-10')
    expect(nextDueDate(new Date(2026, 10, 5), 10)).toBe('2026-11-10')
    expect(nextDueDate(new Date(2026, 11, 21), 10)).toBe('2027-01-10')
  })
})

describe('salaryFallsShort', () => {
  it('avisa quando o primeiro salário depois do mergulho não cobre', () => {
    expect(nextDay('2026-12-31')).toBe('2027-01-01')
    expect(salaryFallsShort({ until: '2026-12-14', exitDate: '2026-12-15', inLimit: 100 })).toBe(false)
    expect(salaryFallsShort({ until: '2026-12-14', exitDate: '2026-12-30', inLimit: 100 })).toBe(true)
    expect(salaryFallsShort({ until: '2026-12-14', exitDate: null, inLimit: 100 })).toBe(true)
    expect(salaryFallsShort({ until: '2026-12-14', exitDate: null, inLimit: 0 })).toBe(false)
  })
})
