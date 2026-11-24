import { formatBRL } from '../../../shared/lib/format'
import { currentMonth, isMonth } from '../../../shared/lib/months'
import type { Transaction } from '../api'

export const FILTER_KEYS = ['tipo', 'categoria', 'sub', 'essencial', 'forma', 'conta', 'q'] as const

/** "todos" na URL desliga o filtro de mês; ausente ou inválido vale o mês atual. */
export function monthFilter(params: URLSearchParams, now = currentMonth()): string | null {
  const raw = params.get('mes')
  if (raw === 'todos') return null
  return isMonth(raw) ? raw : now
}

/** Converte os parâmetros da URL (pt-BR) na query da API, descartando o que está vazio. */
export function toApiQuery(params: URLSearchParams, now = currentMonth()): string {
  const out = new URLSearchParams()
  const month = monthFilter(params, now)
  if (month) out.set('month', month)

  const map: Record<string, string> = {
    tipo: 'kind',
    categoria: 'category',
    sub: 'subcategory',
    essencial: 'essential',
    forma: 'payment_method',
    conta: 'account',
    q: 'q',
  }
  for (const key of FILTER_KEYS) {
    const value = params.get(key)?.trim()
    if (value) out.set(map[key], value)
  }
  const page = Number(params.get('pagina'))
  if (Number.isInteger(page) && page > 1) out.set('page', String(page))
  return out.toString()
}

/**
 * As receitas previstas (salário e entradas recorrentes que ainda não caíram) só aparecem quando o filtro deixa ver as
 * receitas do ciclo inteiro: com mês escolhido, tipo vazio ou Receita e nenhum outro filtro (elas não têm conta, forma de
 * pagamento nem descrição do banco para filtrar).
 */
export function showsExpectedIncome(params: URLSearchParams, now = currentMonth()): boolean {
  if (!monthFilter(params, now)) return false
  const kind = params.get('tipo')?.trim()
  if (kind && kind !== 'INCOME') return false
  const category = params.get('categoria')?.trim()
  if (category && category !== INCOME_GROUP) return false
  return (['sub', 'essencial', 'forma', 'conta', 'q'] as const).every((k) => !params.get(k)?.trim())
}

/**
 * As despesas previstas (saídas recorrentes que ainda não saíram: aluguel, seguro, financiamento) seguem a mesma regra das
 * receitas: mês escolhido, tipo vazio ou Despesa e nenhum outro filtro (não têm categoria, conta nem descrição do banco).
 */
export function showsExpectedExpenses(params: URLSearchParams, now = currentMonth()): boolean {
  if (!monthFilter(params, now)) return false
  const kind = params.get('tipo')?.trim()
  if (kind && kind !== 'EXPENSE') return false
  // Conta fixa (aluguel, seguro, financiamento) é essencial: continua prevista quando o filtro é "Essencial".
  const essential = params.get('essencial')?.trim()
  if (essential && essential !== 'true') return false
  return (['categoria', 'sub', 'forma', 'conta', 'q'] as const).every((k) => !params.get(k)?.trim())
}

/**
 * Valor com sinal e o que ele significa. Investimento e caixinha não são gasto nem ganho: ficam neutros, mas com o sinal do
 * ponto de vista da reserva — o que foi para ela é "+", o que saiu dela (resgate) é "-".
 */
export function describeAmount(t: Pick<Transaction, 'kind' | 'transferDirection' | 'amount'> & { category?: string }) {
  const value = formatBRL(t.amount)
  switch (t.kind) {
    case 'INCOME':
      return { text: `+${value}`, tone: 'in' as const, caption: 'Receita' }
    case 'TRANSFER': {
      const out = t.transferDirection === 'IN'
      const caption = t.category === 'RESERVE' ? (out ? 'Saiu da caixinha' : 'Foi para a caixinha') : out ? 'Resgate' : 'Aplicação'
      return { text: `${out ? '-' : '+'}${value}`, tone: 'neutral' as const, caption }
    }
    default:
      return { text: `-${value}`, tone: 'out' as const, caption: 'Despesa' }
  }
}

/** "2026-09-03" → "qui, 03/09" (dia da semana calculado em UTC, sem deslocar o dia). */
export function formatDayLabel(iso: string): string {
  const [y, m, d] = iso.split('-').map(Number)
  const weekday = new Date(Date.UTC(y, m - 1, d)).toLocaleDateString('pt-BR', { weekday: 'short', timeZone: 'UTC' }).replace('.', '')
  return `${weekday}, ${String(d).padStart(2, '0')}/${String(m).padStart(2, '0')}`
}

/** Chave do grupo que junta todas as receitas no agrupamento por categoria (vem da API). */
export const INCOME_GROUP = 'INCOME'

export interface IncomeSource {
  key: string
  label: string
  total: number
  items: Transaction[]
}

const titleCase = (s: string) =>
  s
    .toLowerCase()
    .split(' ')
    .map((w) => (['de', 'da', 'do', 'das', 'dos', 'e'].includes(w) ? w : w.charAt(0).toUpperCase() + w.slice(1)))
    .join(' ')

/**
 * De onde veio uma receita: salário, Pix/TED de uma pessoa, rendimentos ou, sem padrão conhecido, a própria descrição.
 * Maiúsculas e espaços não separam fontes ("Pix recebido MONICA" e "Pix recebido Monica" são a mesma).
 */
export function incomeSource(t: Pick<Transaction, 'category' | 'description'>): { key: string; label: string } {
  if (t.category === 'SALARY') return { key: 'salary', label: 'Salário' }
  const desc = t.description.replace(/\s+/g, ' ').trim()
  if (/^(rendimentos?|remunera[cç][aã]o aplica[cç][aã]o)/i.test(desc)) return { key: 'yield', label: 'Rendimentos' }
  const person = desc.match(/^(?:pix|ted|transfer[eê]ncia)(?: recebid[oa])? (.+)$/i)
  if (person) {
    const name = titleCase(person[1])
    return { key: `from:${name.toLowerCase()}`, label: `${/^ted/i.test(desc) ? 'TED' : 'Pix'} de ${name}` }
  }
  if (!desc) return { key: 'other', label: 'Sem descrição' }
  return { key: `desc:${desc.toLowerCase()}`, label: desc }
}

/** Receitas agrupadas por fonte, da maior para a menor. */
export function incomeSources(items: Transaction[]): IncomeSource[] {
  const map = new Map<string, IncomeSource>()
  for (const t of items) {
    const { key, label } = incomeSource(t)
    const g = map.get(key) ?? { key, label, total: 0, items: [] }
    g.total += t.amount
    g.items.push(t)
    map.set(key, g)
  }
  return [...map.values()].sort((a, b) => b.total - a.total)
}

/** Parâmetros de um filtro extra (categoria ou dia) sobre a query atual, para listar o detalhe de um grupo. */
export function withGroupFilter(query: string, key: 'category' | 'day', value: string): string {
  const q = new URLSearchParams(query)
  if (key === 'category' && value === INCOME_GROUP) {
    // "Entradas" junta todas as receitas, de qualquer categoria.
    q.set('kind', 'INCOME')
  } else if (key === 'category') {
    // As categorias mostram só gastos e investimentos: as receitas estão em Entradas.
    q.set(key, value)
    if (!q.get('kind')) q.set('exclude_kind', 'INCOME')
  } else {
    q.set(key, value)
  }
  q.delete('page')
  q.set('limit', '100')
  if (key === 'day') q.delete('month')
  return q.toString()
}

/** Instante ISO do banco → "18:27", sempre no horário de São Paulo; vazio se ausente ou inválido. */
export function formatBankTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit', timeZone: 'America/Sao_Paulo' })
}

/** "2026-09-03" → "03/09/2026", sem passar por Date (evita deslocamento de fuso). */
export function formatDay(iso: string): string {
  const [y, m, d] = iso.split('-')
  return `${d}/${m}/${y}`
}

/**
 * Aplicação e resgate são o mesmo dinheiro indo e voltando (ex.: caixinha): somar os dois não diz nada. O que importa é o
 * líquido, aplicado − resgatado: quanto ficou guardado (ou saiu da reserva) no período. Com reserve, os textos falam da
 * caixinha.
 */
export function investSummary(applied: number, redeemed: number, reserve = false) {
  const net = Math.round((applied - redeemed) * 100) / 100
  const signed = `${net < 0 ? '-' : '+'}${formatBRL(Math.abs(net))}`
  if (reserve) {
    return {
      net,
      signed,
      headline: net >= 0 ? `Guardou ${formatBRL(net)} na caixinha` : `Tirou ${formatBRL(-net)} da caixinha`,
      detail: `foi para a caixinha ${formatBRL(applied)} · saiu ${formatBRL(redeemed)}`,
    }
  }
  return {
    net,
    signed,
    headline: net >= 0 ? `Guardou ${formatBRL(net)}` : `Tirou da reserva ${formatBRL(-net)}`,
    detail: `aplicado ${formatBRL(applied)} · resgatado ${formatBRL(redeemed)}`,
  }
}
