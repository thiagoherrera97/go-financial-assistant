import type { FlowInput, RecurringFlow } from '../api'

/** O formulário guarda tudo como texto; toFlowInput valida e converte (a API valida de novo). */
export interface FlowForm {
  in: boolean
  name: string
  party: string
  amount: string
  step: string
  maxAmount: string
  day: string
  startMonth: string
  installments: string
  payeeMatch: string
}

export const emptyFlowForm = (isIn: boolean, startMonth: string): FlowForm => ({
  in: isIn, name: '', party: '', amount: '', step: '', maxAmount: '', day: '10', startMonth, installments: '', payeeMatch: '',
})

export const flowToForm = (f: RecurringFlow): FlowForm => ({
  in: f.in, name: f.name, party: f.party, amount: String(f.amount), step: f.step ? String(f.step) : '',
  maxAmount: f.maxAmount ? String(f.maxAmount) : '', day: String(f.day), startMonth: f.startMonth,
  installments: f.installments ? String(f.installments) : '', payeeMatch: f.payeeMatch,
})

const decimal = (s: string) => (s.trim() === '' ? 0 : s.includes(',') ? Number(s.replace(/\./g, '').replace(',', '.')) : Number(s))

export function toFlowInput(f: FlowForm): { input: FlowInput } | { error: string } {
  const name = f.name.trim()
  if (!name) return { error: 'Dê um nome' }
  const amount = decimal(f.amount)
  if (!(amount > 0)) return { error: 'Informe o valor' }
  const step = decimal(f.step)
  const maxAmount = decimal(f.maxAmount)
  if (!(step >= 0) || !(maxAmount >= 0)) return { error: 'Aumento e teto não podem ser negativos' }
  if (maxAmount > 0 && maxAmount < amount) return { error: 'O teto não pode ser menor que o valor' }
  const day = Number(f.day)
  if (!Number.isInteger(day) || day < 1 || day > 31) return { error: 'O dia vai de 1 a 31' }
  if (!/^\d{4}-\d{2}$/.test(f.startMonth)) return { error: 'Informe o mês de início' }
  const installments = f.installments.trim() === '' ? 0 : Number(f.installments)
  if (!Number.isInteger(installments) || installments < 0 || installments > 120) return { error: 'Vezes: de 1 a 120, ou vazio se não tem fim' }
  return {
    input: { in: f.in, name, party: f.party.trim(), amount, step, maxAmount, day, startMonth: f.startMonth, installments, payeeMatch: f.payeeMatch.trim() },
  }
}

/** Uma linha que descreve o combinado: "R$ 113 todo dia 10 · 10x" ou "sobe R$ 100 por mês até R$ 1.300". */
export function flowRule(f: Pick<RecurringFlow, 'step' | 'maxAmount' | 'installments' | 'day'>, money: (v: number) => string): string {
  const parts = [`todo dia ${f.day}`]
  if (f.step > 0) parts.push(`sobe ${money(f.step)} por mês${f.maxAmount > 0 ? ` até ${money(f.maxAmount)}` : ''}`)
  if (f.installments > 0) parts.push(`${f.installments}x`)
  return parts.join(' · ')
}
