import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from './request'

/** true = essencial, false = supérfluo, null = depende do caso (a revisar). */
export type Essential = boolean | null

export interface SubcategoryInfo {
  code: string
  category: string
  label: string
  essential: Essential
}

export interface SubcategoryCatalog {
  /** Categorias que uma despesa pode ter. */
  categories: { value: string; label: string }[]
  subcategories: SubcategoryInfo[]
}

/** O catálogo não muda enquanto o app roda: busca uma vez. */
export const useSubcategoryCatalog = () =>
  useQuery({
    queryKey: ['subcategories'],
    queryFn: () => request<SubcategoryCatalog>('/api/subcategories'),
    staleTime: Infinity,
  })

export interface SubcategorySpendItem {
  category: string
  categoryLabel: string
  /** Vazio em categorias sem subcategoria (Pessoas, Outros). */
  subcategory: string
  subcategoryLabel: string
  essential: Essential
  count: number
  total: number
  /** Média por mês dos 3 meses anteriores, no mesmo grupo. */
  prevAverage: number
  /** Parte do total que é parcela de compra já feita (cortar o gasto não a tira dos próximos meses). */
  installments: number
}

export interface SubcategorySpend {
  month: string
  essential: number
  superfluous: number
  unknown: number
  items: SubcategorySpendItem[]
}

export const useSubcategorySpend = (month: string) =>
  useQuery({
    queryKey: ['subcategory-spend', month],
    queryFn: () => request<SubcategorySpend>(`/api/spending/subcategories?month=${month}`),
    placeholderData: keepPreviousData,
  })

export interface ClassificationInput {
  category: string
  /** Vazio = o app escolhe. */
  subcategory: string
  /** null = vale o da subcategoria. */
  essential: Essential
  /** Grava a regra do estabelecimento: corrige as outras despesas dele e as próximas. */
  applyToAll: boolean
}

export const setClassification = (id: string, input: ClassificationInput) =>
  request<{ others: number }>(`/api/transactions/${id}/classification`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })

export const essentialLabel = (e: Essential) => (e === null ? 'A revisar' : e ? 'Essencial' : 'Supérfluo')
