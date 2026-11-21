import { deltaPercent } from '../../../shared/lib/format'
import type { SubcategorySpend, SubcategorySpendItem } from '../../../shared/api/subcategories'

export interface CutItem extends SubcategorySpendItem {
  /** Variação contra a média dos 3 meses anteriores; null sem histórico. */
  delta: number | null
  /** Quanto sobraria em um ano cortando metade, no ritmo deste mês. */
  halfPerYear: number
  /** Transações deste grupo no mês. */
  link: string
}

/** Link para as transações de um grupo no mês, com os mesmos filtros da tela de Transações. */
export function groupLink(month: string, item: Pick<SubcategorySpendItem, 'category' | 'subcategory' | 'essential'>): string {
  const q = new URLSearchParams({ mes: month, categoria: item.category, vista: 'lista' })
  if (item.subcategory) q.set('sub', item.subcategory)
  q.set('essencial', item.essential === null ? 'unknown' : String(item.essential))
  return `/transacoes?${q.toString()}`
}

const withMeta = (month: string, i: SubcategorySpendItem): CutItem => ({
  ...i,
  delta: deltaPercent(i.total, i.prevAverage),
  halfPerYear: Math.max(i.total, 0) * 6,
  link: groupLink(month, i),
})

/** O supérfluo do mês, do que mais pesa para o que menos pesa: é por onde começar a cortar. */
export function superfluousRanking(data: SubcategorySpend): CutItem[] {
  return data.items
    .filter((i) => i.essential === false && i.total > 0)
    .sort((a, b) => b.total - a.total)
    .map((i) => withMeta(data.month, i))
}

/** O que o app não soube dizer se é essencial ou supérfluo (Outros, Pessoas, "Outras contas"...). */
export function toReview(data: SubcategorySpend): CutItem[] {
  return data.items
    .filter((i) => i.essential === null && i.total > 0)
    .sort((a, b) => b.total - a.total)
    .map((i) => withMeta(data.month, i))
}

/** Divisão do mês em essencial, supérfluo e a revisar, em fração do total (0 a 1). */
export function split(data: SubcategorySpend) {
  const total = data.essential + data.superfluous + data.unknown
  const share = (v: number) => (total > 0 ? v / total : 0)
  return {
    total,
    essential: share(data.essential),
    superfluous: share(data.superfluous),
    unknown: share(data.unknown),
  }
}
