export class ApiError extends Error {
  status: number
  /** O app ainda não foi configurado: a API só atende o setup ({"setup": "required"}) e o front leva para /setup. */
  setupRequired: boolean
  constructor(status: number, message: string, setupRequired = false) {
    super(message)
    this.status = status
    this.setupRequired = setupRequired
  }
}

export const isSetupRequired = (error: unknown) => error instanceof ApiError && error.setupRequired

// Enquanto o backend reinicia (um redeploy leva ~1–2s), o nginx responde 502/503/504 ou a conexão cai.
// Só repete métodos idempotentes: repetir um PUT/GET/DELETE não duplica nada, já um POST poderia.
const RETRY_DELAYS_MS = [400, 1000, 2000]
const RETRYABLE_STATUS = new Set([502, 503, 504])
const IDEMPOTENT = new Set(['GET', 'HEAD', 'PUT', 'DELETE'])

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

export async function fetchWithRetry(path: string, init?: RequestInit, delays = RETRY_DELAYS_MS): Promise<Response> {
  const canRetry = IDEMPOTENT.has((init?.method ?? 'GET').toUpperCase())
  for (let attempt = 0; ; attempt++) {
    const last = !canRetry || attempt >= delays.length
    try {
      // Mesma origem: o cookie de sessão (HttpOnly) vai sozinho, e o front nunca vê a senha nem o cookie.
      const res = await fetch(path, { credentials: 'same-origin', ...init })
      if (last || !RETRYABLE_STATUS.has(res.status)) return res
    } catch (err) {
      if (last || (err instanceof DOMException && err.name === 'AbortError')) throw err
    }
    await sleep(delays[attempt])
  }
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetchWithRetry(path, init)
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error ?? 'erro inesperado', body?.setup === 'required')
  return body as T
}

export const postJSON = <T>(path: string, data?: unknown) =>
  request<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data ?? {}),
  })
