import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Check, PencilLine, StickyNote, X } from 'lucide-react'
import { setTransactionNote } from '../api'

const MAX = 300

/** A nota da transação ("o que foi"): mostra em destaque, e um toque abre o campo para escrever ou mudar. */
export function NoteEditor({ id, note }: { id: string; note: string }) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(note)
  const save = useMutation({
    mutationFn: (text: string) => setTransactionNote(id, text),
    onSuccess: () => {
      setEditing(false)
      queryClient.invalidateQueries({ queryKey: ['transactions'] })
      queryClient.invalidateQueries({ queryKey: ['month-sheet'] })
    },
  })

  const edit = () => {
    setDraft(note)
    setEditing(true)
  }
  if (!editing) {
    // Com nota, ela aparece inteira e em destaque (é o "o que foi" da compra); tocar nela abre a edição.
    return note ? (
      <button type="button" className="tx-note-card" onClick={edit} aria-label={`Nota: ${note}. Editar nota`}>
        <StickyNote size={14} aria-hidden="true" />
        <span className="tx-note-card-text">{note}</span>
        <PencilLine size={13} aria-hidden="true" className="tx-note-card-edit" />
      </button>
    ) : (
      <button type="button" className="tx-note-btn" onClick={edit} aria-label="Adicionar nota">
        <PencilLine size={13} aria-hidden="true" /> nota
      </button>
    )
  }
  return (
    <form
      className="tx-note-form"
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate(draft)
      }}
    >
      <input
        className="tx-note-input"
        value={draft}
        maxLength={MAX}
        autoFocus
        placeholder="O que foi? (ex.: insulfilm, minha mãe está ajudando)"
        aria-label="Nota da transação"
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => e.key === 'Escape' && setEditing(false)}
      />
      <button type="submit" className="btn btn-small" disabled={save.isPending} aria-label="Salvar nota">
        <Check size={14} aria-hidden="true" />
      </button>
      <button type="button" className="btn btn-small" onClick={() => setEditing(false)} aria-label="Cancelar">
        <X size={14} aria-hidden="true" />
      </button>
      {save.isError && (
        <span className="tx-note-error" role="alert">
          Não salvou. Tente de novo.
        </span>
      )}
    </form>
  )
}
