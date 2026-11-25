import { createContext, useContext } from 'react'

export interface ComposeState {
  open: boolean
  setOpen: (open: boolean) => void
}

export const ComposeContext = createContext<ComposeState>({ open: false, setOpen: () => undefined })

/** Abre e fecha o painel "Lançar" de qualquer tela (o botão fixo do Layout). */
export const useCompose = () => useContext(ComposeContext)
