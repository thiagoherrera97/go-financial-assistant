/** Aba da lista: o valor é o do filtro `essential` da API de transações. */
export type EssentialTab = 'false' | 'true' | 'unknown'

export const ESSENTIAL_TABS: { value: EssentialTab; label: string; empty: string }[] = [
  { value: 'false', label: 'Supérfluos', empty: 'supérflua' },
  { value: 'true', label: 'Essenciais', empty: 'essencial' },
  { value: 'unknown', label: 'A revisar', empty: 'a revisar' },
]

/** Despesas do mês de uma aba, do maior valor para o menor. */
export const essentialQuery = (month: string, tab: EssentialTab, limit: number) =>
  new URLSearchParams({ month, kind: 'EXPENSE', essential: tab, sort: 'amount', limit: String(limit) }).toString()
