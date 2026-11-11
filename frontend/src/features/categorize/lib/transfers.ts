import { formatBRL } from '../../../shared/lib/format'
import type { TransferDecision, TransferItem } from '../api'

export const DECISION_LABEL: Record<TransferDecision, string> = {
  OWN: 'Entre contas minhas',
  PERSON: 'Para uma pessoa',
  PAYMENT: 'Pagamento',
}

/** "03/11/2026" a partir de AAAA-MM-DD (sem passar por Date, que mudaria o dia pelo fuso). */
export function formatDay(day: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day)
  return m ? `${m[3]}/${m[2]}/${m[1]}` : day
}

/** "Saída · R$ 250,50 · 03/11/2026 · Itaú 1234" */
export function transferSummary(t: TransferItem): string {
  const parts = [t.direction === 'in' ? 'Entrada' : 'Saída', formatBRL(t.amount), formatDay(t.date)]
  if (t.account) parts.push(t.account)
  return parts.join(' · ')
}

/** "Pagamento" só vale para saídas: uma entrada não é um pagamento seu. */
export const canBePayment = (t: TransferItem) => t.direction === 'out'

/** O que a decisão fez, em palavras, para a lista de decididas. */
export function decisionNote(t: TransferItem, categoryLabel: (value: string) => string): string {
  switch (t.decision) {
    case 'OWN':
      return 'Não conta mais como gasto nem como renda.'
    case 'PERSON':
      return 'Categoria: Pessoas.'
    case 'PAYMENT':
      return `Categoria: ${categoryLabel(t.category)}.`
    default:
      return ''
  }
}

/** Texto do aviso quando a lista vem cortada. */
export const truncatedNote = (shown: number, total: number) => (total > shown ? `Mostrando as ${shown} mais recentes de ${total}.` : '')

/** Soma das transferências da lista (valor absoluto, entradas e saídas separadas). */
export function totals(items: TransferItem[]): { out: number; in: number } {
  return items.reduce((acc, t) => (t.direction === 'in' ? { ...acc, in: acc.in + t.amount } : { ...acc, out: acc.out + t.amount }), { out: 0, in: 0 })
}
