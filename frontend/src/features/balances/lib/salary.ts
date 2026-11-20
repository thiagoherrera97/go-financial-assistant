import type { SalaryPart } from '../api'

const WEEKDAYS = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb']

/** "13/11 (sex)" a partir de AAAA-MM-DD, sem passar por fuso. */
export function shortDay(iso: string): string {
  const [y, m, d] = iso.split('-').map(Number)
  const wd = new Date(Date.UTC(y, m - 1, d)).getUTCDay()
  return `${String(d).padStart(2, '0')}/${String(m).padStart(2, '0')} (${WEEKDAYS[wd]})`
}

/** Linha de uma parte: quando caiu ou quando deve cair, e se foi antecipada. */
export function salaryPartText(p: SalaryPart): string {
  if (p.received && p.receivedDate) return `caiu ${shortDay(p.receivedDate)}`
  const day = Number(p.expectedDate.slice(8, 10))
  const moved = day !== p.day ? ' · antecipado (fim de semana ou feriado)' : ''
  return `cai ${shortDay(p.expectedDate)}${moved}`
}
