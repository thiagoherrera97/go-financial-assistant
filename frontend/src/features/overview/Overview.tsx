import { QueryState } from '../../shared/components/QueryState'
import { MonthFilter } from '../../shared/components/MonthFilter'
import { TrendChart } from '../../shared/components/TrendChart'
import { StatTile } from '../../shared/components/StatTile'
import { INCOME_EXPENSE, toMonthRows } from '../../shared/lib/chart'
import { formatBRL, formatMonthTitle } from '../../shared/lib/format'
import { shiftMonth } from '../../shared/lib/months'
import { useMonth } from '../../shared/lib/useMonth'
import { useSummary, useTimeseries } from '../../shared/api/totals'
import { Link } from 'react-router'
import { formatShare, shouldWarnOtherShare } from './lib/otherShare'
import { signTone } from '../../shared/lib/tone'
import { useExpectedExpenses, useExpectedIncome } from '../../shared/api/expectedIncome'

export default function Overview() {
  const { month, setMonth, now } = useMonth()
  const summary = useSummary(month)
  // Salário e entradas recorrentes do ciclo que ainda não caíram: entram em Receitas e no Saldo.
  const expected = useExpectedIncome(month, true)
  const toPay = useExpectedExpenses(month, true)
  const series = useTimeseries(shiftMonth(month, -11), month)

  return (
    <>
      <h1 className="page-title">{formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} warnPartial />

      <QueryState query={summary}>
        {(data) => (
          <>
            <OtherShareNotice share={data.otherShare} />
            <Tiles data={data} stale={summary.isPlaceholderData} partial={month === now} expected={expected.data?.total ?? 0} toPay={toPay.data?.total ?? 0} />
          </>
        )}
      </QueryState>

      <QueryState query={series}>
        {(data) => <TrendChart series={INCOME_EXPENSE} title="Receitas e despesas por mês" rows={toMonthRows(data)} stale={series.isPlaceholderData} />}
      </QueryState>
    </>
  )
}

/** "Outros" alto deixa os gráficos por categoria sem sentido: leva à tela onde se classifica e se revisam as transferências. */
function OtherShareNotice({ share }: { share: number | null }) {
  if (!shouldWarnOtherShare(share)) return null
  return (
    <p className="notice" role="note">
      &quot;Outros&quot; é {formatShare(share)} das despesas deste mês, então os gráficos por categoria dizem pouco.{' '}
      <Link className="link" to="/classificar">
        Classificar contas e revisar transferências
      </Link>
    </p>
  )
}

function Tiles({ data, stale, partial, expected, toPay }: { data: NonNullable<ReturnType<typeof useSummary>['data']>; stale: boolean; partial: boolean; expected: number; toPay: number }) {
  const { current: c, previous: p } = data
  const balance = c.income + expected - c.expense - toPay
  const invested = c.applied - c.redeemed

  return (
    <div className={`tiles chart-body ${stale ? 'is-stale' : ''}`}>
      <StatTile
        label="Receitas"
        tone="good"
        value={formatBRL(c.income + expected)}
        note={expected > 0 ? `${formatBRL(expected)} ainda a receber` : undefined}
        compare={partial ? undefined : { current: c.income, previous: p.income, previousMonth: p.month, upIsGood: true }}
      />
      <StatTile
        label="Despesas"
        tone="bad"
        value={formatBRL(c.expense + toPay)}
        note={toPay > 0 ? `${formatBRL(toPay)} ainda a pagar` : undefined}
        compare={partial ? undefined : { current: c.expense, previous: p.expense, previousMonth: p.month, upIsGood: false }}
      />
      <StatTile
        label="Saldo do mês"
        tone={signTone(balance)}
        value={formatBRL(balance)}
        note={expected > 0 || toPay > 0 ? `Com o que já caiu: ${formatBRL(c.income - c.expense)}` : undefined}
        compare={partial ? undefined : { current: balance, previous: p.income - p.expense, previousMonth: p.month, upIsGood: true }}
      />
      <StatTile
        label="Em conta"
        tone={data.bankBalance === null ? undefined : signTone(data.bankBalance)}
        to="/"
        value={data.bankBalance === null ? '—' : formatBRL(data.bankBalance)}
        note={data.bankBalance === null ? 'Sem contas do Open Finance' : 'Contas correntes, hoje'}
      />
      <StatTile
        label="Investido no mês"
        value={formatBRL(invested)}
        note={`Aplicado ${formatBRL(c.applied)} · Resgatado ${formatBRL(c.redeemed)}`}
      />
    </div>
  )
}
