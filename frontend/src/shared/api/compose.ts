import { postJSON, request } from './request'

/** Respostas do mesmo cérebro do chat (Telegram), na ordem. */
export interface ComposeResult {
  replies: string[]
}

export const composeText = (text: string) => postJSON<ComposeResult>('/api/compose', { text })

/** Áudio cru no corpo, com Content-Type audio/*: o backend transcreve com o Gemini e segue como texto. */
export const composeAudio = (audio: Blob) =>
  request<ComposeResult>('/api/compose/audio', {
    method: 'POST',
    headers: { 'Content-Type': audio.type || 'audio/wav' },
    body: audio,
  })
