import type { SalaryPart } from '../api'
import { shortDay } from '../../../shared/lib/format'

/** Linha de uma parte: quando caiu ou quando deve cair, e se foi antecipada. */
export function salaryPartText(p: SalaryPart): string {
  if (p.received && p.receivedDate) return `caiu ${shortDay(p.receivedDate)}`
  const day = Number(p.expectedDate.slice(8, 10))
  const moved = day !== p.day ? ' · antecipado (fim de semana ou feriado)' : ''
  return `cai ${shortDay(p.expectedDate)}${moved}`
}

/** Quanto a parte veio acima do combinado (hora extra no mesmo crédito), ou null se não veio acima. */
export function salaryBonus(p: SalaryPart): number | null {
  if (!p.received || p.receivedAmount === null) return null
  const diff = Math.round((p.receivedAmount - p.amount) * 100) / 100
  return diff >= 0.5 ? diff : null
}
