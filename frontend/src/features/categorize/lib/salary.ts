import { formatBRL } from '../../../shared/lib/format'
import { shiftMonth } from '../../../shared/lib/months'
import type { SalaryDecision, SalaryKind, SalaryReviewItem } from '../api'

const MONTHS = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro']
const monthName = (month: string) => MONTHS[Number(month.split('-')[1]) - 1] ?? month

/** Os extras, na ordem da tela. */
const EXTRAS: { value: SalaryKind; label: string }[] = [
  { value: 'OVERTIME', label: 'Hora extra' },
  { value: 'PLR', label: 'PLR' },
  { value: 'THIRTEENTH', label: '13º (ou adiantamento do 13º)' },
  { value: 'VACATION', label: 'Férias' },
  { value: 'OTHER', label: 'Outro extra' },
]

/**
 * As respostas para um crédito: o salário do mês inteiro (o mês do palpite, ou o seguinte, para o salário pago adiantado),
 * cada parte do salário e os extras. O valor codifica a decisão ("MONTH:2026-12", "PART:15", "PLR").
 */
export function salaryOptions(item: SalaryReviewItem, parts: { day: number; amount: number }[]) {
  const next = shiftMonth(item.month, 1)
  return [
    { value: `MONTH:${item.month}`, label: `Salário de ${monthName(item.month)} inteiro` },
    { value: `MONTH:${next}`, label: `Salário de ${monthName(next)} inteiro (adiantado)` },
    ...parts.map((p) => ({ value: `PART:${p.day}`, label: `Parte do dia ${p.day} (${formatBRL(p.amount)})` })),
    ...EXTRAS,
  ]
}

/** A opção que já vem marcada: o palpite do app. Um extra começa sem escolha (o app não sabe se é PLR ou hora extra). */
export function initialChoice(item: SalaryReviewItem): string {
  if (item.suggest === 'MONTH') return `MONTH:${item.month}`
  if (item.suggest === 'PART' && item.day) return `PART:${item.day}`
  return ''
}

/** O corpo da decisão a partir da opção escolhida; month é o ciclo do crédito, para uma parte. */
export function decisionOf(value: string, month?: string): SalaryDecision {
  const [kind, arg] = value.split(':')
  if (kind === 'MONTH') return { kind, month: arg }
  if (kind === 'PART') return { kind, month, day: Number(arg) }
  return { kind: kind as SalaryKind }
}

/** "Salário de dezembro", "Parte do dia 30 de dezembro", "PLR": a decisão tomada, para a lista de decididos. */
export function decidedNote(d: { kind: SalaryKind; kindLabel: string; month?: string; day?: number }): string {
  if (d.kind === 'MONTH' && d.month) return `Salário de ${monthName(d.month)} inteiro`
  if (d.kind === 'PART' && d.month) return `Parte do dia ${d.day} de ${monthName(d.month)}`
  return d.kindLabel
}
