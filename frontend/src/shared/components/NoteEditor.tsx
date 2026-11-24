import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Check, PencilLine, X } from 'lucide-react'
import { setTransactionNote } from '../api/notes'

const MAX = 300

/** A nota da transação ("o que foi"): o botão abre o campo para escrever ou mudar. */
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
      queryClient.invalidateQueries({ queryKey: ['review'] })
    },
  })

  const edit = () => {
    setDraft(note)
    setEditing(true)
  }
  if (!editing) {
    // A nota em si é o título da linha (TransactionTable); aqui fica só o botão de escrever ou editar.
    return (
      <button type="button" className="tx-note-btn" onClick={edit} aria-label={note ? 'Editar nota' : 'Adicionar nota'}>
        <PencilLine size={13} aria-hidden="true" /> {note ? 'editar nota' : 'nota'}
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
