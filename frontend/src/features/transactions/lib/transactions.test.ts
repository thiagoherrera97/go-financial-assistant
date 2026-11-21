import { describe, expect, it } from 'vitest'
import { describeAmount, investSummary, formatBankTime, formatDay, formatDayLabel, incomeSources, INCOME_GROUP, monthFilter, toApiQuery, withGroupFilter } from './transactions'
import type { Transaction } from '../api'

const q = (s: string) => new URLSearchParams(s)
const plain = (s: string) => s.replace(/ /g, ' ')

describe('toApiQuery', () => {
  it('usa o mês atual por padrão e traduz os filtros', () => {
    expect(toApiQuery(q(''), '2026-10')).toBe('month=2026-10')
    expect(toApiQuery(q('mes=2026-09&tipo=EXPENSE&categoria=FOOD&forma=PIX&q=padaria&pagina=3'), '2026-10')).toBe(
      'month=2026-09&kind=EXPENSE&category=FOOD&payment_method=PIX&q=padaria&page=3',
    )
  })

  it('"todos" remove o mês e vazios são descartados', () => {
    expect(toApiQuery(q('mes=todos&tipo=&q=%20%20'), '2026-10')).toBe('')
  })

  it('ignora mês e página inválidos', () => {
    expect(toApiQuery(q('mes=lixo&pagina=-2'), '2026-10')).toBe('month=2026-10')
    expect(toApiQuery(q('pagina=1.5'), '2026-10')).toBe('month=2026-10')
  })

  it('codifica o texto da busca', () => {
    expect(toApiQuery(q('mes=todos&q=a%26b%3Dc'), '2026-10')).toBe('q=a%26b%3Dc')
  })
})

describe('monthFilter', () => {
  it('distingue "todos" de mês atual', () => {
    expect(monthFilter(q('mes=todos'), '2026-10')).toBeNull()
    expect(monthFilter(q(''), '2026-10')).toBe('2026-10')
  })
})

describe('describeAmount', () => {
  it('dá sinal e legenda por tipo', () => {
    expect(describeAmount({ kind: 'EXPENSE', amount: 45 })).toMatchObject({ tone: 'out', caption: 'Despesa' })
    expect(plain(describeAmount({ kind: 'EXPENSE', amount: 45 }).text)).toBe('-R$ 45,00')
    expect(plain(describeAmount({ kind: 'INCOME', amount: 10 }).text)).toBe('+R$ 10,00')
    expect(describeAmount({ kind: 'TRANSFER', transferDirection: 'IN', amount: 1 })).toMatchObject({ tone: 'neutral', caption: 'Resgate' })
    expect(plain(describeAmount({ kind: 'TRANSFER', transferDirection: 'IN', amount: 1 }).text)).toBe('-R$ 1,00')
    expect(plain(describeAmount({ kind: 'TRANSFER', transferDirection: 'OUT', amount: 1, category: 'RESERVE' }).text)).toBe('+R$ 1,00')
    expect(describeAmount({ kind: 'TRANSFER', transferDirection: 'OUT', amount: 1 }).caption).toBe('Aplicação')
    expect(describeAmount({ kind: 'TRANSFER', transferDirection: 'OUT', amount: 1, category: 'RESERVE' }).caption).toBe('Foi para a caixinha')
    expect(describeAmount({ kind: 'TRANSFER', transferDirection: 'IN', amount: 1, category: 'RESERVE' }).caption).toBe('Saiu da caixinha')
  })
})

describe('formatDay', () => {
  it('formata sem deslocar o dia', () => {
    expect(formatDay('2026-09-03')).toBe('03/09/2026')
  })
})

describe('formatDayLabel', () => {
  it('mostra o dia da semana sem deslocar por fuso', () => {
    expect(formatDayLabel('2026-09-03')).toBe('qui, 03/09')
    expect(formatDayLabel('2026-01-01')).toBe('qui, 01/01')
  })
})

describe('withGroupFilter', () => {
  it('acrescenta a categoria, pede até 100 itens e volta à primeira página', () => {
    expect(withGroupFilter('month=2026-09&kind=EXPENSE&page=3', 'category', 'FOOD')).toBe('month=2026-09&kind=EXPENSE&category=FOOD&limit=100')
  })

  it('o filtro por dia troca o mês', () => {
    expect(withGroupFilter('month=2026-09&q=padaria', 'day', '2026-09-03')).toBe('q=padaria&day=2026-09-03&limit=100')
  })
})

describe('investSummary', () => {
  it('mostra o líquido: o mesmo dinheiro indo e voltando não se soma', () => {
    expect(investSummary(1171.33, 5627.18)).toMatchObject({ net: -4455.85, headline: 'Tirou da reserva R$\u00a04.455,85' })
    expect(investSummary(800, 800).headline).toBe('Guardou R$\u00a00,00')
    expect(investSummary(1171.33, 5627.18).signed).toBe('-R$\u00a04.455,85')
    expect(investSummary(2171.33, 6178.11, true)).toMatchObject({ headline: 'Tirou R$\u00a04.006,78 da caixinha', detail: 'foi para a caixinha R$\u00a02.171,33 · saiu R$\u00a06.178,11' })
    expect(investSummary(1000, 200).detail).toBe('aplicado R$\u00a01.000,00 · resgatado R$\u00a0200,00')
  })
})

describe('incomeSources', () => {
  const tx = (description: string, amount: number, category = 'OTHER') =>
    ({ id: description + amount, description, amount, category }) as unknown as Transaction

  it('junta por fonte, ignorando maiúsculas e espaços, da maior para a menor', () => {
    const groups = incomeSources([
      tx('Pix recebido Monica Nunes de Andrade', 100),
      tx('Pix recebido MONICA NUNES DE ANDRADE', 50),
      tx('LIQUIDO DE VENCIMENTO   45007649848', 5000, 'SALARY'),
      tx('Rendimentos', 0.15),
      tx('REMUNERACAO APLICACAO AUTOMATICA', 0.01),
      tx('TED RECEBIDA   Monica Nunes Herrera', 2000),
      tx('Reembolso Compra garantida', 30),
    ])
    expect(groups.map((g) => [g.label, g.items.length])).toEqual([
      ['Salário', 1],
      ['TED de Monica Nunes Herrera', 1],
      ['Pix de Monica Nunes de Andrade', 2],
      ['Reembolso Compra garantida', 1],
      ['Rendimentos', 2],
    ])
  })
})

describe('withGroupFilter em Entradas', () => {
  it('Entradas filtra por tipo; as categorias tiram as receitas', () => {
    expect(new URLSearchParams(withGroupFilter('month=2026-11', 'category', INCOME_GROUP)).get('kind')).toBe('INCOME')
    const other = new URLSearchParams(withGroupFilter('month=2026-11', 'category', 'OTHER'))
    expect(other.get('category')).toBe('OTHER')
    expect(other.get('exclude_kind')).toBe('INCOME')
  })
})

describe('formatBankTime', () => {
  it('mostra a hora e o minuto do banco', () => {
    expect(formatBankTime('2026-11-17T21:27:58Z')).toMatch(/^\d{2}:\d{2}$/)
  })
  it('fica vazio sem hora ou com valor inválido', () => {
    expect(formatBankTime(undefined)).toBe('')
    expect(formatBankTime('ontem')).toBe('')
  })
})
