import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Check, Tag, X } from 'lucide-react'
import { essentialLabel, setClassification, useSubcategoryCatalog } from '../../../shared/api/subcategories'
import type { Transaction } from '../api'

type EssentialChoice = 'auto' | 'yes' | 'no'

/**
 * Corrige a categoria, a subcategoria e o essencial/supérfluo de uma despesa (ex.: o lava-rápido que o banco chama de
 * "marmitex"). Com "aplicar às outras", vale para as despesas com a mesma descrição e para as próximas.
 */
export function ClassificationEditor({ tx }: { tx: Transaction }) {
  const queryClient = useQueryClient()
  const catalog = useSubcategoryCatalog()
  const [editing, setEditing] = useState(false)
  const [category, setCategory] = useState(tx.category)
  const [subcategory, setSubcategory] = useState(tx.subcategory ?? '')
  const [essential, setEssential] = useState<EssentialChoice>('auto')
  const [applyToAll, setApplyToAll] = useState(false)
  const [done, setDone] = useState('')

  const save = useMutation({
    mutationFn: () =>
      setClassification(tx.id, {
        category,
        subcategory,
        essential: essential === 'auto' ? null : essential === 'yes',
        applyToAll,
      }),
    onSuccess: ({ others }) => {
      setEditing(false)
      setDone(others > 0 ? `+${others} ${others === 1 ? 'outra corrigida' : 'outras corrigidas'}` : '')
      for (const key of ['transactions', 'transaction-groups', 'subcategory-spend', 'breakdown', 'month-sheet', 'summary', 'categorize']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    },
  })

  if (tx.kind !== 'EXPENSE') return null

  if (!editing) {
    return (
      <>
        <button
          type="button"
          className="tx-note-btn"
          onClick={() => {
            setCategory(tx.category)
            setSubcategory(tx.subcategory ?? '')
            setEssential('auto')
            setApplyToAll(false)
            setDone('')
            setEditing(true)
          }}
          aria-label="Corrigir categoria"
        >
          <Tag size={13} aria-hidden="true" /> categoria
        </button>
        {done && <span className="tx-note-text">{done}</span>}
      </>
    )
  }

  const subs = (catalog.data?.subcategories ?? []).filter((s) => s.category === category)
  const auto = subs.find((s) => s.code === subcategory)
  return (
    <form
      className="tx-class-form"
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate()
      }}
    >
      <label className="field">
        <span className="sr-only">Categoria</span>
        <select
          value={category}
          autoFocus
          onChange={(e) => {
            setCategory(e.target.value)
            setSubcategory('')
          }}
        >
          {(catalog.data?.categories ?? [{ value: tx.category, label: tx.categoryLabel }]).map((c) => (
            <option key={c.value} value={c.value}>
              {c.label}
            </option>
          ))}
        </select>
      </label>
      {subs.length > 0 && (
        <label className="field">
          <span className="sr-only">Subcategoria</span>
          <select value={subcategory} onChange={(e) => setSubcategory(e.target.value)}>
            <option value="">Subcategoria automática</option>
            {subs.map((s) => (
              <option key={s.code} value={s.code}>
                {s.label} ({essentialLabel(s.essential).toLowerCase()})
              </option>
            ))}
          </select>
        </label>
      )}
      <div className="segmented" role="radiogroup" aria-label="Essencial ou supérfluo">
        {(
          [
            ['auto', auto ? `Padrão (${essentialLabel(auto.essential).toLowerCase()})` : 'Padrão'],
            ['yes', 'Essencial'],
            ['no', 'Supérfluo'],
          ] as const
        ).map(([value, label]) => (
          <button key={value} type="button" role="radio" aria-checked={essential === value} className="btn btn-small" onClick={() => setEssential(value)}>
            {label}
          </button>
        ))}
      </div>
      <label className="check tx-class-all">
        <input type="checkbox" checked={applyToAll} onChange={(e) => setApplyToAll(e.target.checked)} />
        Aplicar às outras com esta descrição e às próximas
      </label>
      <span className="tx-class-actions">
        <button type="submit" className="btn btn-small" disabled={save.isPending} aria-label="Salvar categoria">
          <Check size={14} aria-hidden="true" /> Salvar
        </button>
        <button type="button" className="btn btn-small" onClick={() => setEditing(false)} aria-label="Cancelar">
          <X size={14} aria-hidden="true" />
        </button>
      </span>
      {save.isError && (
        <span className="tx-note-error" role="alert">
          Não salvou. Tente de novo.
        </span>
      )}
    </form>
  )
}
