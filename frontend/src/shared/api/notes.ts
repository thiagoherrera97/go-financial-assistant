import { request } from './request'

/** Grava a nota da transação; vazia apaga. */
export const setTransactionNote = (id: string, note: string) =>
  request<{ note: string }>(`/api/transactions/${id}/note`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ note }),
  })
