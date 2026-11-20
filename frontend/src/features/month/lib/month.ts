import type { SheetCard } from '../api'

/** Linhas extras da fatura além das categorias: previstas, ajuste do banco e estornos (só as que existem). */
export function cardNotes(c: SheetCard): { label: string; amount: number; hint: string }[] {
  const out: { label: string; amount: number; hint: string }[] = []
  if (c.forecast !== 0) out.push({ label: 'Parcelas previstas', amount: c.forecast, hint: 'O banco ainda não mandou; somem quando a parcela real chega.' })
  if (c.adjustment !== 0)
    out.push({
      label: c.adjustment > 0 ? 'Não detalhado pelo banco' : 'Créditos não detalhados',
      amount: c.adjustment,
      hint: 'Diferença para o total da fatura informado pelo banco.',
    })
  return out
}

/** "Sobrou" ou "Faltou", como na planilha. */
export const leftoverLabel = (v: number) => (v >= 0 ? 'Sobra' : 'Faltou')
