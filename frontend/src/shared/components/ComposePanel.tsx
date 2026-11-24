import { useQueryClient } from '@tanstack/react-query'
import { Mic, Plus, SendHorizontal, Square, X } from 'lucide-react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ApiError } from '../api/request'
import { composeAudio, composeText, type ComposeResult } from '../api/compose'
import { chatText } from '../lib/chatText'
import { useCompose } from '../lib/composeContext'
import { toWav16k } from '../lib/wav'

interface Line {
  from: 'me' | 'bot'
  text: string
}

const EXAMPLES = ['gastei 32,90 no mercado', 'nota 393 dia 1/11: jantar pizza', 'tênis 600 em 3x no cartão']

// Acima disso o envio fica pesado e o Gemini demora; o backend aceita até 15 MB (uns 7 minutos de WAV).
const MAX_RECORD_SECONDS = 180

function errorText(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 413) return 'Áudio grande demais. Grave um mais curto.'
    if (error.status === 429) return 'Muitas mensagens seguidas. Espere um minuto.'
    if (error.status === 401) return 'Sessão expirada. Entre de novo.'
  }
  return 'Não consegui mandar agora. Tente de novo.'
}

/**
 * Painel "Lançar": o mesmo chat do Telegram dentro do app (gasto, parcelado, nota, por texto ou voz), para lançar na hora o
 * que ainda não chegou do banco. A conversa é à parte da do Telegram e some ao recarregar a página.
 */
export function ComposePanel() {
  const { open, setOpen } = useCompose()
  const queryClient = useQueryClient()
  const [lines, setLines] = useState<Line[]>([])
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [recording, setRecording] = useState(false)
  const [seconds, setSeconds] = useState(0)
  const [micError, setMicError] = useState<string | null>(null)
  const recorder = useRef<MediaRecorder | null>(null)
  const input = useRef<HTMLTextAreaElement>(null)
  const list = useRef<HTMLOListElement>(null)

  useEffect(() => {
    if (!open) return
    // Só no computador: no celular, focar abre o teclado sem a pessoa pedir; lá ele abre ao tocar no campo.
    if (window.matchMedia('(pointer: fine)').matches) input.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !recording && setOpen(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, recording, setOpen])

  useEffect(() => {
    list.current?.lastElementChild?.scrollIntoView({ block: 'end' })
  }, [lines, busy])

  // No iPhone o teclado cobre a página sem encolher a tela: o painel sobe junto e cabe no que sobra visível.
  const [keyboard, setKeyboard] = useState<{ bottom: number; height: number } | null>(null)
  useEffect(() => {
    const vv = window.visualViewport
    if (!open || !vv) return
    const update = () => {
      const bottom = Math.max(0, window.innerHeight - vv.height - vv.offsetTop)
      setKeyboard(bottom > 0 ? { bottom, height: vv.height } : null)
    }
    update()
    vv.addEventListener('resize', update)
    vv.addEventListener('scroll', update)
    return () => {
      vv.removeEventListener('resize', update)
      vv.removeEventListener('scroll', update)
    }
  }, [open])

  useEffect(() => {
    if (!recording) return
    const t = setInterval(() => setSeconds((s) => s + 1), 1000)
    return () => clearInterval(t)
  }, [recording])

  useEffect(() => {
    if (recording && seconds >= MAX_RECORD_SECONDS) recorder.current?.stop()
  }, [recording, seconds])

  async function send(call: () => Promise<ComposeResult>, mine?: string) {
    if (mine) setLines((l) => [...l, { from: 'me', text: mine }])
    setBusy(true)
    try {
      const { replies } = await call()
      setLines((l) => [...l, ...replies.map((r) => ({ from: 'bot' as const, text: r }))])
      // Um lançamento ou nota muda Transações, o Ciclo e os totais: tudo é buscado de novo.
      void queryClient.invalidateQueries()
    } catch (e) {
      setLines((l) => [...l, { from: 'bot', text: '⚠️ ' + errorText(e) }])
    } finally {
      setBusy(false)
    }
  }

  function submit(e?: FormEvent) {
    e?.preventDefault()
    const t = text.trim()
    if (!t || busy) return
    setText('')
    void send(() => composeText(t), t)
  }

  async function startRecording() {
    setMicError(null)
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
      setMicError('Este navegador não grava áudio. Escreva a mensagem.')
      return
    }
    let stream: MediaStream
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch {
      setMicError('Sem acesso ao microfone. Libere nas permissões do navegador.')
      return
    }
    const rec = new MediaRecorder(stream)
    const chunks: Blob[] = []
    rec.ondataavailable = (ev) => ev.data.size > 0 && chunks.push(ev.data)
    rec.onstop = async () => {
      stream.getTracks().forEach((t) => t.stop())
      setRecording(false)
      recorder.current = null
      const raw = new Blob(chunks, { type: rec.mimeType || 'audio/webm' })
      if (raw.size === 0) return
      // WAV de 16 kHz é aceito pelo Gemini em qualquer navegador; se não der para converter, vai o original.
      const audio = (await toWav16k(raw)) ?? raw
      void send(() => composeAudio(audio), '🎙️ Áudio')
    }
    recorder.current = rec
    setSeconds(0)
    setRecording(true)
    rec.start()
  }

  if (!open) return null

  const clock = `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`

  return (
    <>
      <div className="compose-scrim" onClick={() => !recording && setOpen(false)} aria-hidden="true" />
      <section
        className="compose"
        role="dialog"
        aria-modal="true"
        aria-labelledby="compose-title"
        style={keyboard ? { bottom: keyboard.bottom, maxHeight: keyboard.height - 8 } : undefined}
      >
        <header className="compose-head">
          <h2 id="compose-title">Lançar</h2>
          <button type="button" className="icon-btn" aria-label="Fechar" disabled={recording} onClick={() => setOpen(false)}>
            <X size={20} aria-hidden="true" />
          </button>
        </header>

        <ol className="compose-lines" ref={list} aria-live="polite">
          {lines.length === 0 && (
            <li className="compose-hint">
              <p>Lance o que ainda não chegou do banco ou anote o que foi uma transação, por texto ou voz. Funciona como o Telegram.</p>
              <div className="compose-examples">
                {EXAMPLES.map((ex) => (
                  <button key={ex} type="button" className="compose-example" onClick={() => setText(ex)}>
                    {ex}
                  </button>
                ))}
              </div>
            </li>
          )}
          {lines.map((l, i) => (
            <li key={i} className={`compose-line compose-${l.from}`}>
              {chatText(l.text)}
            </li>
          ))}
          {busy && <li className="compose-line compose-bot compose-busy">Pensando…</li>}
        </ol>

        {micError && <p className="compose-error error">{micError}</p>}

        <form className="compose-form" onSubmit={submit}>
          {recording ? (
            <p className="compose-recording" role="status">
              <span className="compose-dot" aria-hidden="true" /> Gravando {clock}
            </p>
          ) : (
            <textarea
              ref={input}
              rows={1}
              value={text}
              placeholder="gastei 50 no almoço…"
              aria-label="Mensagem"
              disabled={busy}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  submit()
                }
              }}
            />
          )}
          {recording ? (
            <button type="button" className="btn btn-primary btn-icon compose-stop" aria-label="Parar e enviar" onClick={() => recorder.current?.stop()}>
              <Square size={16} aria-hidden="true" />
            </button>
          ) : text.trim() ? (
            <button type="submit" className="btn btn-primary btn-icon" aria-label="Enviar" disabled={busy}>
              <SendHorizontal size={18} aria-hidden="true" />
            </button>
          ) : (
            <button type="button" className="btn btn-icon" aria-label="Gravar áudio" disabled={busy} onClick={() => void startRecording()}>
              <Mic size={18} aria-hidden="true" />
            </button>
          )}
        </form>
      </section>
    </>
  )
}

/** Botão fixo no canto: abre o painel de qualquer tela. */
export function ComposeFab() {
  const { open, setOpen } = useCompose()
  if (open) return null
  return (
    <button type="button" className="compose-fab btn btn-primary" onClick={() => setOpen(true)}>
      <Plus size={18} aria-hidden="true" /> Lançar
    </button>
  )
}
