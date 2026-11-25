import { money } from '../lib/balances'
import { useCountUp } from '../lib/useCountUp'
import { signTone } from '../../../shared/lib/tone'

interface Props {
  value: number
  hidden: boolean
}

/** Saldo verde quando positivo, vermelho quando negativo; oculto, sem cor (a cor entregaria o sinal). */
const moneyClass = (value: number, hidden: boolean) => {
  const tone = hidden ? undefined : signTone(value)
  return tone ? `bal-money is-${tone}` : 'bal-money'
}

/** Valor em reais que conta até o número ao carregar; oculto, mostra R$ •••• (e não anima). */
export function Money({ value, hidden, duration }: Props & { duration?: number }) {
  const shown = useCountUp(value, duration)
  return <span className={moneyClass(value, hidden)}>{money(hidden ? value : shown, hidden)}</span>
}

/** Sem animação, para valores pequenos e secundários. bad: dinheiro que vai sair (fatura, dívida), em vermelho. */
export function StaticMoney({ value, hidden, bad = false }: Props & { bad?: boolean }) {
  return <span className={bad && !hidden && value > 0.004 ? 'bal-money is-bad' : 'bal-money'}>{money(value, hidden)}</span>
}
