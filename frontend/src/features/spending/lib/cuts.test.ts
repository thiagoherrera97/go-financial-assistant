import { describe, expect, it } from 'vitest'
import { groupLink, split, superfluousRanking, toReview } from './cuts'
import type { SubcategorySpend } from '../../../shared/api/subcategories'

const item = (over: Partial<SubcategorySpend['items'][number]>) => ({
  category: 'FOOD',
  categoryLabel: 'Alimentação',
  subcategory: 'FOOD_DELIVERY',
  subcategoryLabel: 'Delivery',
  essential: false,
  count: 1,
  total: 100,
  prevAverage: 0,
  installments: 0,
  ...over,
})

const data: SubcategorySpend = {
  month: '2026-11',
  essential: 600,
  superfluous: 300,
  unknown: 100,
  items: [
    item({ subcategory: 'FOOD_RESTAURANT', subcategoryLabel: 'Restaurante e bar', total: 100, prevAverage: 200 }),
    item({ total: 200, prevAverage: 100 }),
    item({ category: 'MARKET', subcategory: 'MARKET_GROCERY', essential: true, total: 600 }),
    item({ category: 'OTHER', subcategory: '', subcategoryLabel: 'Outros', essential: null, total: 100 }),
  ],
}

describe('superfluousRanking', () => {
  it('ordena o supérfluo do maior para o menor, com variação e economia de cortar metade', () => {
    const r = superfluousRanking(data)
    expect(r.map((i) => i.subcategory)).toEqual(['FOOD_DELIVERY', 'FOOD_RESTAURANT'])
    expect(r[0].delta).toBeCloseTo(1)
    expect(r[1].delta).toBeCloseTo(-0.5)
    expect(r[0].halfPerYear).toBe(1200)
  })
  it('parcela de compra já feita não entra na economia de cortar metade', () => {
    const r = superfluousRanking({
      ...data,
      items: [
        item({ subcategory: 'SHOPPING_BEAUTY', total: 1338, installments: 1338 }),
        item({ subcategory: 'FOOD_RESTAURANT', total: 698.43, installments: 7.76 }),
      ],
    })
    expect(r[0].halfPerYear).toBe(0)
    expect(r[0].onlyInstallments).toBe(true)
    expect(r[1].halfPerYear).toBeCloseTo(4144.02)
    expect(r[1].onlyInstallments).toBe(false)
  })
  it('sem histórico a variação é nula', () => {
    expect(superfluousRanking({ ...data, items: [item({ prevAverage: 0 })] })[0].delta).toBeNull()
  })
})

describe('toReview', () => {
  it('lista só o que está a revisar', () => {
    expect(toReview(data).map((i) => i.category)).toEqual(['OTHER'])
  })
})

describe('groupLink', () => {
  it('leva às transações do grupo no mês', () => {
    expect(groupLink('2026-11', { category: 'FOOD', subcategory: 'FOOD_DELIVERY', essential: false })).toBe(
      '/transacoes?mes=2026-11&categoria=FOOD&vista=lista&sub=FOOD_DELIVERY&essencial=false',
    )
    expect(groupLink('2026-11', { category: 'OTHER', subcategory: '', essential: null })).toBe(
      '/transacoes?mes=2026-11&categoria=OTHER&vista=lista&essencial=unknown',
    )
  })
})

describe('split', () => {
  it('divide o mês em frações', () => {
    expect(split(data)).toEqual({ total: 1000, essential: 0.6, superfluous: 0.3, unknown: 0.1 })
  })
  it('mês vazio não divide por zero', () => {
    expect(split({ ...data, essential: 0, superfluous: 0, unknown: 0 }).essential).toBe(0)
  })
})
