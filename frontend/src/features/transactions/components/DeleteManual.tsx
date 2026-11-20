import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { deleteTransaction } from '../api'

/** Apagar um lançamento manual avulso, com confirmação na própria linha. */
export function DeleteManual({ id }: { id: string }) {
  const queryClient = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const remove = useMutation({
    mutationFn: () => deleteTransaction(id),
    onSuccess: () => queryClient.invalidateQueries(),
  })

  if (!confirming) {
    return (
      <button type="button" className="tx-note-btn" onClick={() => setConfirming(true)} aria-label="Apagar lançamento manual">
        <Trash2 size={13} aria-hidden="true" /> apagar
      </button>
    )
  }
  return (
    <span className="tx-delete">
      Apagar este lançamento manual?
      <button type="button" className="btn btn-small" disabled={remove.isPending} onClick={() => remove.mutate()}>
        Apagar
      </button>
      <button type="button" className="btn btn-small" onClick={() => setConfirming(false)}>
        Manter
      </button>
      {remove.isError && (
        <span className="tx-note-error" role="alert">
          Não apagou. Tente de novo.
        </span>
      )}
    </span>
  )
}
