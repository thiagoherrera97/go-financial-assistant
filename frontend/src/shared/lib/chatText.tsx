import type { ReactNode } from 'react'

/** Mostra a formatação do chat (*negrito* e _itálico_, como no Telegram) sem HTML: o texto vem do servidor e nunca vira markup. */
export function chatText(text: string): ReactNode[] {
  return text.split(/(\*[^*\n]+\*|_[^_\n]+_)/g).map((part, i) => {
    if (part.length > 2 && part.startsWith('*') && part.endsWith('*')) return <strong key={i}>{part.slice(1, -1)}</strong>
    if (part.length > 2 && part.startsWith('_') && part.endsWith('_')) return <em key={i}>{part.slice(1, -1)}</em>
    return part
  })
}
