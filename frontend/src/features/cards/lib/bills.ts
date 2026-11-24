import { formatBRL } from '../../../shared/lib/format'
import type { BillOrigin, BillState, CardBill } from '../api'

const MONTHS = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']

/** "dez/26" a partir de AAAA-MM-DD. */
export function billMonth(due: string): string {
  const [y, m] = due.split('-')
  return `${MONTHS[Number(m) - 1]}/${y.slice(2)}`
}

/** "10/12" a partir de AAAA-MM-DD. */
export function dayMonth(iso: string): string {
  const [, m, d] = iso.split('-')
  return `${d}/${m}`
}

export const STATE_LABEL: Record<BillState, string> = { CLOSED: 'fechada', OPEN: 'em aberto', FUTURE: 'futura' }

export const ORIGIN_LABEL: Record<BillOrigin, string | null> = {
  BANK: null,
  MANUAL: 'lançada à mão',
  FORECAST: 'prevista',
  GAP: 'ajuste',
  REPLACED: 'trocada por lançamento manual',
}

/**
 * A fatura que a tela abre: a pedida (pelo vencimento, se existir); senão a aberta; senão a fechada mais recente; senão a
 * última da lista. As faturas vêm do vencimento mais antigo para o mais novo.
 */
export function pickBill(bills: CardBill[], wanted: string | null): number {
  if (bills.length === 0) return -1
  const asked = wanted === null ? -1 : bills.findIndex((b) => b.due === wanted)
  if (asked >= 0) return asked
  const open = bills.findIndex((b) => b.state === 'OPEN')
  if (open >= 0) return open
  for (let i = bills.length - 1; i >= 0; i--) if (bills[i].state === 'CLOSED') return i
  return bills.length - 1
}

export type Check = { tone: 'good' | 'bad' | 'neutral'; title: string; detail: string }

/**
 * A conferência da fatura com o banco, em palavras. first: é a primeira fatura do histórico do app, que costuma começar no
 * meio dela (as compras de antes do começo do histórico não vieram).
 */
export function billCheck(b: CardBill, first = false): Check {
  if (b.bankTotal === null) {
    const what = b.state === 'FUTURE' ? 'Fatura futura: por enquanto só parcelas.' : 'Fatura em aberto: ainda entram compras.'
    return {
      tone: 'neutral',
      title: 'Ainda não dá para conferir',
      detail: `${what} O banco informa o total quando ela fechar; até lá, o valor é a soma dos lançamentos.`,
    }
  }
  if (b.difference === 0) {
    return { tone: 'good', title: 'Bate com o banco', detail: `Os lançamentos do app somam ${formatBRL(b.known)}, o mesmo que o banco cobrou.` }
  }
  if (b.difference > 0 && first) {
    return {
      tone: 'neutral',
      title: `O banco cobrou ${formatBRL(b.difference)} a mais`,
      detail: `É a primeira fatura do histórico do app: as compras de antes do começo do histórico não vieram do banco, então a diferença é esperada.`,
    }
  }
  if (b.difference > 0) {
    return {
      tone: 'bad',
      title: `O banco cobrou ${formatBRL(b.difference)} a mais`,
      detail: `O banco cobrou ${formatBRL(b.bankTotal)}, mas os lançamentos do app somam ${formatBRL(b.known)}. A diferença entra como “não detalhado no Open Finance”: confira no app do banco se falta alguma compra (juros, IOF, anuidade).`,
    }
  }
  return {
    tone: 'bad',
    title: `Os lançamentos passam ${formatBRL(-b.difference)} do banco`,
    detail: `O banco cobrou ${formatBRL(b.bankTotal)}, mas os lançamentos do app somam ${formatBRL(b.known)}. Pode ser compra repetida, estorno que não chegou ou parcela prevista que o banco não cobrou.`,
  }
}
