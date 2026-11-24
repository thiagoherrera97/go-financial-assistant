import { useState, type ReactNode } from 'react'
import { ComposeContext } from './composeContext'

export function ComposeProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  return <ComposeContext.Provider value={{ open, setOpen }}>{children}</ComposeContext.Provider>
}
