import { describe, expect, it } from 'vitest'
import { salaryPartText, shortDay } from './salary'

describe('salário', () => {
  it('mostra o dia com a semana', () => {
    expect(shortDay('2026-11-13')).toBe('13/11 (sex)')
    expect(shortDay('2026-11-30')).toBe('30/11 (seg)')
  })

  it('diz quando caiu ou quando cai, e avisa a antecipação', () => {
    const base = { day: 15, amount: 5200, receivedAmount: null, receivedDate: null, received: false }
    expect(salaryPartText({ ...base, expectedDate: '2026-11-13' })).toBe('cai 13/11 (sex) · antecipado (fim de semana ou feriado)')
    expect(salaryPartText({ ...base, day: 30, expectedDate: '2026-11-30' })).toBe('cai 30/11 (seg)')
    expect(salaryPartText({ ...base, expectedDate: '2026-11-13', received: true, receivedDate: '2026-11-13', receivedAmount: 5200 })).toBe(
      'caiu 13/11 (sex)',
    )
  })
})
