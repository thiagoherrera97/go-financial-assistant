/**
 * Converte o áudio gravado (webm no Chrome, mp4 no Safari) em WAV mono de 16 kHz: um formato que o Gemini sempre aceita,
 * leve o bastante para voz (cerca de 2 MB por minuto). Devolve null se o navegador não conseguir decodificar.
 */
export async function toWav16k(recording: Blob): Promise<Blob | null> {
  try {
    const AudioCtx = window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
    const ctx = new AudioCtx()
    const decoded = await ctx.decodeAudioData(await recording.arrayBuffer())
    void ctx.close()
    const rate = 16000
    const offline = new OfflineAudioContext(1, Math.max(1, Math.ceil(decoded.duration * rate)), rate)
    const src = offline.createBufferSource()
    src.buffer = decoded
    src.connect(offline.destination)
    src.start()
    const mono = (await offline.startRendering()).getChannelData(0)
    return encodeWav(mono, rate)
  } catch {
    return null
  }
}

export function encodeWav(samples: Float32Array, rate: number): Blob {
  const buf = new ArrayBuffer(44 + samples.length * 2)
  const v = new DataView(buf)
  const str = (at: number, s: string) => [...s].forEach((c, i) => v.setUint8(at + i, c.charCodeAt(0)))
  str(0, 'RIFF')
  v.setUint32(4, 36 + samples.length * 2, true)
  str(8, 'WAVE')
  str(12, 'fmt ')
  v.setUint32(16, 16, true)
  v.setUint16(20, 1, true) // PCM
  v.setUint16(22, 1, true) // mono
  v.setUint32(24, rate, true)
  v.setUint32(28, rate * 2, true)
  v.setUint16(32, 2, true)
  v.setUint16(34, 16, true)
  str(36, 'data')
  v.setUint32(40, samples.length * 2, true)
  samples.forEach((s, i) => {
    const x = Math.max(-1, Math.min(1, s))
    v.setInt16(44 + i * 2, x < 0 ? x * 0x8000 : x * 0x7fff, true)
  })
  return new Blob([buf], { type: 'audio/wav' })
}
