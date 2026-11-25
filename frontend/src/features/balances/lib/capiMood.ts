import type { CapiMood } from '../../../shared/components/Capi'
import type { BalanceCard, Balances } from '../api'

export interface CapiState {
  mood: CapiMood
  line: string
}

const dueWhen = (card: BalanceCard): string => {
  const d = card.daysToDue
  if (d === null) return 'está apertando'
  if (d === 0) return 'vence hoje'
  if (d === 1) return 'vence amanhã'
  return `vence em ${d} dias`
}

/**
 * Como a Capi reage ao Painel. Preocupada com conta no vermelho ou cartão em nível crítico (vencimento ou limite);
 * atenta com cartão em atenção; feliz com saldo positivo e nada apertando. Com os valores ocultos fica neutra:
 * o humor entregaria o que está escondido.
 */
export function capiMood(data: Balances, hidden: boolean): CapiState {
  if (hidden) return { mood: 'neutral', line: 'Valores escondidos. Tô de olho mesmo assim.' }

  const total = data.totalInAccount
  const cards = data.institutions.flatMap((i) => i.cards)
  const owes = (c: BalanceCard) => c.invoice > 0.004

  if (total !== null && total < 0) return { mood: 'worried', line: 'Conta no vermelho. Bora segurar os gastos?' }

  const dueCritical = cards.find((c) => owes(c) && c.dueLevel === 'critical')
  if (dueCritical) return { mood: 'worried', line: `A fatura do ${dueCritical.name} ${dueWhen(dueCritical)}.` }
  const limitCritical = cards.find((c) => c.usageLevel === 'critical')
  if (limitCritical) return { mood: 'worried', line: `O ${limitCritical.name} está quase no limite.` }

  const attention = cards.find((c) => (owes(c) && c.dueLevel === 'warning') || c.usageLevel === 'warning')
  if (attention) return { mood: 'neutral', line: `De olho no ${attention.name}.` }

  if (total !== null && total > 0) return { mood: 'happy', line: 'Tudo em dia por aqui!' }
  return { mood: 'neutral', line: 'Tudo calmo por aqui.' }
}
