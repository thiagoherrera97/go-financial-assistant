import { formatBRL } from '../../../shared/lib/format'
import type { MonthSheet, SheetCard } from '../api'

/** Linhas extras da fatura além das categorias: previstas, ajuste do banco e estornos (só as que existem). */
export function cardNotes(c: SheetCard): { label: string; amount: number; hint: string }[] {
  const out: { label: string; amount: number; hint: string }[] = []
  if (c.forecast !== 0) out.push({ label: 'Parcelas previstas', amount: c.forecast, hint: 'O banco ainda não mandou; somem quando a parcela real chega.' })
  if (c.adjustment !== 0)
    out.push({
      label: c.adjustment > 0 ? 'Não detalhado pelo banco' : 'Créditos não detalhados',
      amount: c.adjustment,
      hint: 'Diferença para o total da fatura informado pelo banco.',
    })
  return out
}

/** "Sobrou" ou "Faltou", como na planilha. */
export const leftoverLabel = (v: number) => (v >= 0 ? 'Sobra' : 'Faltou')

const shortDay = (iso: string) => {
  const [y, m, d] = iso.split('-').map(Number)
  const month = new Date(Date.UTC(y, m - 1, d)).toLocaleDateString('pt-BR', { month: 'short', timeZone: 'UTC' }).replace('.', '')
  return `${d}/${month}`
}

/** Período do ciclo do salário, ex.: "13/nov – 14/dez". */
export const formatCycle = (startsOn: string, endsOn: string) => `${shortDay(startsOn)} – ${shortDay(endsOn)}`

/** A sobra do ciclo: soma o que falta receber (salário e entradas recorrentes), desconta o que falta pagar (saídas
 * recorrentes) e soma o que veio do ciclo anterior, quando há. */
export function sheetLeftover(
  s: Pick<MonthSheet, 'leftover' | 'leftoverWithPending' | 'pendingTotal' | 'carryover' | 'leftoverWithCarryover'> & { toPayTotal?: number },
): { value: number; note: string } {
  const toPay = (s.toPayTotal ?? 0) > 0
  const forecast = s.pendingTotal > 0 && toPay ? 'o que falta receber e pagar' : toPay ? 'o que falta pagar' : 'o que falta receber'
  const pending = s.pendingTotal > 0 || toPay
  if (s.carryover != null) {
    const what = `contando ${formatBRL(s.carryover)} do ciclo anterior`
    return { value: s.leftoverWithCarryover, note: pending ? `${what} e ${forecast}` : what }
  }
  if (pending) return { value: s.leftoverWithPending, note: `Com o que já caiu: ${formatBRL(s.leftover)}` }
  return { value: s.leftover, note: 'entradas − despesas' }
}
