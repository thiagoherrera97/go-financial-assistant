import { describe, expect, it } from 'vitest'
import { salaryBonus, salaryPartText } from './salary'
import { shortDay } from '../../../shared/lib/format'

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

  it('mostra o que veio acima do combinado', () => {
    const p = { day: 30, amount: 4366.62, expectedDate: '2026-09-30', received: true, receivedDate: '2026-09-30', receivedAmount: 9213.81 }
    expect(salaryBonus(p)).toBe(4847.19)
    expect(salaryBonus({ ...p, receivedAmount: 4366.62 })).toBeNull()
    expect(salaryBonus({ ...p, received: false, receivedAmount: null })).toBeNull()
  })
})
