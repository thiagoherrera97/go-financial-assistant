import type { CSSProperties } from 'react'
import { Link } from 'react-router'
import { CalendarClock, CreditCard, OctagonAlert, TriangleAlert } from 'lucide-react'
import { dueText, LEVEL_LABEL, money, nextBillText, safeHex } from '../lib/balances'
import { formatPercent } from '../../../shared/lib/format'
import { StaticMoney } from './Money'
import type { BalanceCard, Level } from '../api'

const BRANDS: Record<string, string> = { MASTERCARD: 'Mastercard', VISA: 'Visa', ELO: 'Elo', AMEX: 'American Express', HIPERCARD: 'Hipercard' }

function LevelIcon({ level }: { level: Level }) {
  if (level === 'critical') return <OctagonAlert size={14} aria-hidden="true" />
  return <TriangleAlert size={14} aria-hidden="true" />
}

/**
 * Um cartão de crédito, sempre sozinho (cartões não são somados). Cartão escuro com a luz da cor do banco no
 * canto; o texto é branco. Atenção e crítico aparecem com ícone e texto.
 */
export function CreditCardTile({ card, color, hidden }: { card: BalanceCard; color: string | null; hidden: boolean }) {
  const brand = safeHex(color)
  const due = dueText(card)
  const nextBill = nextBillText(card)
  const hasLimit = card.limit !== null && card.available !== null && card.usedRatio !== null && card.used !== null
  // A cor do banco só ilumina o canto de um cartão escuro; o texto é sempre branco, então serve qualquer cor.
  const style = { '--brand': brand } as CSSProperties

  return (
    <div className="bal-card" style={style}>
      <div className="bal-card-top">
        <span>
          <CreditCard size={16} aria-hidden="true" /> {card.name}
          {card.last4 && ` · final ${card.last4}`}
        </span>
        {card.brand && <span>{BRANDS[card.brand] ?? card.brand}</span>}
      </div>
      <div>
        <p className="bal-card-invoice">
          <StaticMoney value={card.invoice} hidden={hidden} bad />
        </p>
        <p className="bal-card-sub">saldo devedor · inclui parcelas futuras</p>
        {due && (
          <span className={`bal-pill ${card.dueLevel !== 'ok' ? 'is-alert' : ''}`}>
            {card.dueLevel === 'ok' ? <CalendarClock size={14} aria-hidden="true" /> : <LevelIcon level={card.dueLevel} />} {due}
          </span>
        )}
        {nextBill && (
          <p className="bal-card-sub" title={card.closeEstimated ? 'Fechamento estimado: o banco não informa. Ajuste em Configurações → Sincronização.' : undefined}>
            {nextBill}
          </p>
        )}
        <Link className="bal-card-link" to={`/faturas?cartao=${card.id}`}>
          Ver a fatura e conferir com o banco →
        </Link>
      </div>
      {hasLimit ? (
        <div>
          <div
            className="bal-meter"
            role="meter"
            aria-label="Limite usado"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round((card.usedRatio ?? 0) * 100)}
            aria-valuetext={`${formatPercent(card.usedRatio ?? 0)} do limite usado${LEVEL_LABEL[card.usageLevel] ? `, ${LEVEL_LABEL[card.usageLevel]}` : ''}`}
          >
            <i style={{ width: `${(card.usedRatio ?? 0) * 100}%` }} />
          </div>
          <div className="bal-card-row">
            <span className={card.usageLevel !== 'ok' ? 'bal-card-flag' : undefined}>
              {card.usageLevel !== 'ok' && <LevelIcon level={card.usageLevel} />} Usado {formatPercent(card.usedRatio ?? 0)}
              {card.usageLevel !== 'ok' && ` · ${LEVEL_LABEL[card.usageLevel]}`}
            </span>
            <span>
              Disponível <strong>{money(card.available ?? 0, hidden)}</strong> de {money(card.limit ?? 0, hidden)}
            </span>
          </div>
        </div>
      ) : (
        <p className="bal-card-sub">Limite não informado pelo banco</p>
      )}
    </div>
  )
}
