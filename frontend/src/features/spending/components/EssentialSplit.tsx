import { Link } from 'react-router'
import { QueryState } from '../../../shared/components/QueryState'
import { CategoryChip } from '../../../shared/components/CategoryChip'
import { formatBRL, formatPercent } from '../../../shared/lib/format'
import { useSubcategorySpend } from '../../../shared/api/subcategories'
import type { SubcategorySpend } from '../../../shared/api/subcategories'
import { split, superfluousRanking, toReview } from '../lib/cuts'
import type { CutItem } from '../lib/cuts'

const TOP = 8

/** Quanto do mês foi essencial, supérfluo ou ainda está a revisar, e o supérfluo que mais pesa: por onde cortar. */
export function EssentialSplit({ month }: { month: string }) {
  const query = useSubcategorySpend(month)
  return <QueryState query={query}>{(data) => <Body data={data} stale={query.isPlaceholderData} />}</QueryState>
}

function Body({ data, stale }: { data: SubcategorySpend; stale: boolean }) {
  const s = split(data)
  const cuts = superfluousRanking(data)
  const review = toReview(data)
  if (s.total <= 0) return <p className="state">Sem despesas neste mês.</p>
  const parts = [
    { key: 'essential', label: 'Essencial', value: data.essential, share: s.essential },
    { key: 'superfluous', label: 'Supérfluo', value: data.superfluous, share: s.superfluous },
    { key: 'unknown', label: 'A revisar', value: data.unknown, share: s.unknown },
  ]
  return (
    <div className={`chart-body ${stale ? 'is-stale' : ''}`}>
      <section className="card section-gap" aria-labelledby="h-split">
        <h2 className="chart-title" id="h-split">
          Essencial ou supérfluo
        </h2>
        <div className="split-bar" aria-hidden="true">
          {parts.map((p) => p.share > 0 && <span key={p.key} className={`split-${p.key}`} style={{ width: `${p.share * 100}%` }} />)}
        </div>
        <ul className="split-legend">
          {parts.map((p) => (
            <li key={p.key}>
              <span className={`split-dot split-${p.key}`} aria-hidden="true" />
              {p.label} <strong>{formatBRL(p.value)}</strong> <span className="bar-share">{formatPercent(p.share)}</span>
            </li>
          ))}
        </ul>
      </section>

      <div className="grid-2 section-gap">
        <section className="card" aria-labelledby="h-cuts">
          <h2 className="chart-title" id="h-cuts">
            Onde cortar
          </h2>
          <p className="chart-sub">O supérfluo do mês, do que mais pesa para o que menos pesa. Clique para ver as compras.</p>
          {cuts.length === 0 ? <p className="state">Nada supérfluo neste mês.</p> : <CutList items={cuts.slice(0, TOP)} showSaving />}
        </section>
        <section className="card" aria-labelledby="h-review">
          <h2 className="chart-title" id="h-review">
            A revisar
          </h2>
          <p className="chart-sub">O app não soube dizer se é essencial. Abra, use “categoria” em cada compra e marque.</p>
          {review.length === 0 ? <p className="state">Tudo classificado.</p> : <CutList items={review.slice(0, TOP)} />}
        </section>
      </div>
    </div>
  )
}

function CutList({ items, showSaving = false }: { items: CutItem[]; showSaving?: boolean }) {
  return (
    <ul className="cut-list">
      {items.map((i) => (
        <li key={`${i.category}:${i.subcategory}:${String(i.essential)}`}>
          <Link to={i.link} className="cut-row">
            <CategoryChip category={i.category} size={16} />
            <span className="cut-name">
              <span className="cut-label">{i.subcategoryLabel}</span>
              <span className="cut-meta">
                {i.subcategory ? `${i.categoryLabel} · ` : ''}
                {i.count} {i.count === 1 ? 'compra' : 'compras'}
                {showSaving && i.halfPerYear > 0 && ` · cortar metade: ${formatBRL(i.halfPerYear)}/ano`}
              </span>
            </span>
            <span className="cut-value">
              <strong>{formatBRL(i.total)}</strong>
              {i.delta !== null && (
                <span className={`cut-delta ${i.delta > 0.1 ? 'delta-bad' : i.delta < -0.1 ? 'delta-good' : ''}`}>
                  {i.delta > 0 ? '+' : ''}
                  {formatPercent(i.delta)} vs média
                </span>
              )}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  )
}
