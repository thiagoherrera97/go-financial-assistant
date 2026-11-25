import { freshnessText } from '../../../shared/lib/format'
import { Money, StaticMoney } from './Money'
import { ShareBar } from './ShareBar'
import { Capi } from '../../../shared/components/Capi'
import { capiMood } from '../lib/capiMood'
import type { Balances } from '../api'

export function BalanceHero({ data, hidden, now }: { data: Balances; hidden: boolean; now: Date }) {
  const banks = data.institutions.filter((i) => i.total !== null).length
  const stale = data.institutions.some((i) => i.stale)
  const capi = capiMood(data, hidden)

  return (
    <section className="bal-hero" aria-labelledby="bal-total-label">
      <div className="bal-hero-band">
        {/* A Capi reage ao saldo; a fala é texto de verdade, então quem usa leitor de tela também ouve. */}
        <div className={`bal-capi is-${capi.mood}`} key={capi.mood}>
          <p className="bal-capi-line">{capi.line}</p>
          <Capi mood={capi.mood} size={104} />
        </div>
        <p className="bal-hero-label" id="bal-total-label">
          Total em conta
        </p>
        <p className="bal-hero-value">{data.totalInAccount === null ? '—' : <Money value={data.totalInAccount} hidden={hidden} duration={900} />}</p>
        <p className="bal-hero-note">
          {data.totalInAccount === null
            ? 'Nenhuma conta corrente sincronizada'
            : `${banks} ${banks === 1 ? 'banco' : 'bancos'}`}
          {data.asOf && ` · dados do banco de ${freshnessText(data.asOf, now)}`}
          {stale && ' · há bancos desatualizados'}
        </p>
        {data.openInvoices > 0 && (
          <p className="bal-aside">
            Saldo devedor dos cartões, à parte (não subtraído do total):{' '}
            <strong>
              <StaticMoney value={data.openInvoices} hidden={hidden} bad />
            </strong>
          </p>
        )}
      </div>
      <div className="bal-hero-body">
        <ShareBar institutions={data.institutions} hidden={hidden} />
      </div>
    </section>
  )
}
