import { formatBRL } from '../../../shared/lib/format'
import type { Tone } from '../../../shared/lib/tone'
import type { TransactionHit, Vendor, VendorInput, VendorSummary } from '../api'

/** "23/08/2027" a partir de AAAA-MM-DD, sem passar por fuso. */
export function formatDate(iso: string): string {
  const [y, m, d] = iso.split('-')
  return `${d}/${m}/${y}`
}

/** Hoje em AAAA-MM-DD no fuso do navegador. */
export function todayISO(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`
}

/** Converte "1.234,56", "1234,56", "4.000" (milhar) ou "1234.56" em número; NaN quando não dá. */
export function parseAmount(raw: string): number {
  const s = raw.trim().replace(/\s|R\$/g, '')
  if (!s) return NaN
  const thousandsOnly = /^\d{1,3}(\.\d{3})+$/.test(s)
  const normalized = s.includes(',') || thousandsOnly ? s.replace(/\./g, '').replace(',', '.') : s
  return /^\d+(\.\d{1,2})?$/.test(normalized) ? Number(normalized) : NaN
}

export interface VendorForm {
  name: string
  contracted: string
  dueDate: string
  paid: string
}

export const emptyVendorForm = (): VendorForm => ({ name: '', contracted: '', dueDate: '', paid: '' })

export const vendorToForm = (v: Vendor): VendorForm => ({
  name: v.name,
  contracted: String(v.contracted).replace('.', ','),
  dueDate: v.dueDate ?? '',
  paid: '',
})

/** Valida o formulário como a API valida. withPaid: na criação, "já pago" vira o pagamento inicial. */
export function toVendorInput(f: VendorForm, withPaid: boolean): { input: VendorInput } | { error: string } {
  const name = f.name.trim()
  if (!name || name.length > 60) return { error: 'Informe o nome do fornecedor (até 60 caracteres).' }
  const contracted = parseAmount(f.contracted)
  if (!(contracted > 0)) return { error: 'Informe o valor contratado.' }
  const input: VendorInput = { name, contracted, dueDate: f.dueDate }
  if (withPaid && f.paid.trim()) {
    const paid = parseAmount(f.paid)
    if (!(paid >= 0)) return { error: 'O valor já pago não é um número válido.' }
    if (paid > contracted) return { error: 'O valor já pago passa do contratado.' }
    if (paid > 0) input.paid = paid
  }
  return { input }
}

/** Situação do plano em uma frase, com o tom do aviso. */
export function planStatus(s: VendorSummary): { tone: Tone; text: string } {
  if (s.vendors.length === 0) return { tone: 'ok', text: 'Cadastre os fornecedores para ver quanto guardar por mês.' }
  if (s.remaining === 0) return { tone: 'ok', text: 'Tudo pago ou guardado. 🎉' }
  if (!s.lastDue) return { tone: 'warning', text: 'Informe a data final de pagamento dos fornecedores para calcular quanto guardar por mês.' }
  const late = s.vendors.filter((v) => v.late)
  if (late.length > 0) {
    const names = late.map((v) => v.name).join(', ')
    return { tone: 'critical', text: `Vence antes do próximo aporte: ${names}. Esse valor precisa sair já.` }
  }
  if (s.fits === false && s.surplus !== null)
    return { tone: 'warning', text: `Guardar ${formatBRL(s.perMonth)} por mês passa da sobra média projetada (${formatBRL(s.surplus)}).` }
  if (s.fits === true) return { tone: 'ok', text: `Cabe na sobra média projetada (${formatBRL(s.surplus ?? 0)} por mês).` }
  return { tone: 'ok', text: 'Ainda sem histórico para dizer se cabe na sobra do mês.' }
}

/** Uma transação pode pagar fornecedor se é saída não cancelada (gasto ou transferência de saída). */
export const isOutflow = (t: TransactionHit) =>
  t.status !== 'CANCELLED' && (t.kind === 'EXPENSE' || (t.kind === 'TRANSFER' && t.transferDirection === 'OUT'))

/** Ids de transações já ligadas a algum fornecedor. */
export const linkedIds = (vendors: Vendor[]) =>
  new Set(vendors.flatMap((v) => v.payments.map((p) => p.transactionId).filter((id): id is string => id !== null)))

export const paidRatio = (v: Vendor) => (v.contracted > 0 ? Math.min(1, v.paid / v.contracted) : 0)
