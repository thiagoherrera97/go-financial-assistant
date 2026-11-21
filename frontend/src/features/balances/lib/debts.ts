import type { Debt, DebtInput, DebtKind } from '../api'

/** O formulário guarda tudo como texto; toDebtInput valida e converte (a API valida de novo). */
export interface DebtForm {
  kind: DebtKind
  name: string
  creditor: string
  note: string
  amount: string
  dueDay: string
  accountId: string
  freeDays: string
  installmentAmount: string
  installments: string
  firstDue: string
  payeeMatch: string
  paidManual: string
}

export const DEBT_KINDS: Record<DebtKind, { label: string; hint: string }> = {
  OVERDRAFT: {
    label: 'Coberta pelo limite',
    hint: 'O cheque especial da conta cobre a dívida até o salário cair. O negativo da conta aparece aqui, como esta dívida',
  },
  INSTALLMENTS: {
    label: 'Parcelada',
    hint: 'Parcelas fixas todo mês. O Pix com o nome de quem recebe abate a parcela sozinho',
  },
}

export function emptyDebtForm(kind: DebtKind, firstDue: string): DebtForm {
  return {
    kind, name: '', creditor: '', note: '', amount: '', dueDay: '10', accountId: '', freeDays: '10',
    installmentAmount: '', installments: '10', firstDue, payeeMatch: '', paidManual: '0',
  }
}

export function debtToForm(d: Debt): DebtForm {
  return {
    kind: d.kind, name: d.name, creditor: d.creditor, note: d.note, amount: String(d.originalAmount), dueDay: String(d.dueDay),
    accountId: d.accountId ?? '', freeDays: String(d.freeDays), installmentAmount: d.installmentAmount ? String(d.installmentAmount) : '',
    installments: d.installments ? String(d.installments) : '10', firstDue: d.firstDue ?? '', payeeMatch: d.payeeMatch,
    paidManual: String(d.paidManual),
  }
}

const num = (s: string) => Number(s.replace(/\./g, '').replace(',', '.'))
const decimal = (s: string) => (s.includes(',') ? num(s) : Number(s))

export function toDebtInput(f: DebtForm): { input: DebtInput } | { error: string } {
  const name = f.name.trim()
  if (!name) return { error: 'Dê um nome à dívida' }
  const dueDay = Number(f.dueDay)
  if (!Number.isInteger(dueDay) || dueDay < 1 || dueDay > 31) return { error: 'O dia vai de 1 a 31' }
  const base = { kind: f.kind, name, creditor: f.creditor.trim(), note: f.note.trim(), dueDay }
  if (f.kind === 'OVERDRAFT') {
    const amount = decimal(f.amount)
    if (!(amount > 0)) return { error: 'Informe o valor da dívida' }
    if (!f.accountId) return { error: 'Escolha a conta cujo limite cobre a dívida' }
    const freeDays = Number(f.freeDays)
    if (!Number.isInteger(freeDays) || freeDays < 0 || freeDays > 60) return { error: 'Dias sem juros vão de 0 a 60' }
    return { input: { ...base, originalAmount: amount, accountId: f.accountId, freeDays } }
  }
  const installmentAmount = decimal(f.installmentAmount)
  if (!(installmentAmount > 0)) return { error: 'Informe o valor da parcela' }
  const installments = Number(f.installments)
  if (!Number.isInteger(installments) || installments < 1 || installments > 120) return { error: 'Parcelas vão de 1 a 120' }
  if (!/^\d{4}-\d{2}-\d{2}$/.test(f.firstDue)) return { error: 'Informe a data da primeira parcela' }
  const paidManual = Number(f.paidManual || '0')
  if (!Number.isInteger(paidManual) || paidManual < 0 || paidManual > installments) return { error: 'Parcelas já pagas: de 0 ao total' }
  return {
    input: {
      ...base, originalAmount: Math.round(installmentAmount * installments * 100) / 100, installmentAmount, installments,
      firstDue: f.firstDue, payeeMatch: f.payeeMatch.trim(), paidManual,
    },
  }
}

/** Quanto ainda falta da dívida, para o total do card. */
export function debtOutstanding(d: Debt): number {
  if (d.progress) return d.progress.remaining
  if (d.limit && d.limit.inLimit > 0) return d.limit.inLimit
  return d.forecast?.inLimit ?? 0
}

/** O primeiro dia do próximo mês com o dia dueDay, em AAAA-MM-DD (sugestão para a primeira parcela). */
export function nextDueDate(today: Date, dueDay: number): string {
  let y = today.getFullYear()
  let m = today.getMonth() + 1
  if (today.getDate() >= dueDay) m += 1
  if (m > 12) {
    m = 1
    y += 1
  }
  const last = new Date(y, m, 0).getDate()
  return `${y}-${String(m).padStart(2, '0')}-${String(Math.min(dueDay, last)).padStart(2, '0')}`
}
