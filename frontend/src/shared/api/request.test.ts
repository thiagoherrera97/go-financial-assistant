import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchWithRetry } from './request'

const res = (status: number) => new Response(null, { status })

describe('fetchWithRetry', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('repete um PUT enquanto o backend reinicia (502) e devolve o sucesso', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(res(502)).mockResolvedValueOnce(res(200))
    vi.stubGlobal('fetch', fetchMock)
    const out = await fetchWithRetry('/api/x', { method: 'PUT' }, [0, 0])
    expect(out.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('repete um GET quando a conexão cai', async () => {
    const fetchMock = vi.fn().mockRejectedValueOnce(new TypeError('Failed to fetch')).mockResolvedValueOnce(res(200))
    vi.stubGlobal('fetch', fetchMock)
    expect((await fetchWithRetry('/api/x', undefined, [0])).status).toBe(200)
  })

  it('não repete POST, para não duplicar a ação', async () => {
    const fetchMock = vi.fn().mockResolvedValue(res(502))
    vi.stubGlobal('fetch', fetchMock)
    expect((await fetchWithRetry('/api/x', { method: 'POST' }, [0, 0])).status).toBe(502)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('não repete erros que não são do backend fora do ar', async () => {
    const fetchMock = vi.fn().mockResolvedValue(res(400))
    vi.stubGlobal('fetch', fetchMock)
    expect((await fetchWithRetry('/api/x', { method: 'PUT' }, [0, 0])).status).toBe(400)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('desiste depois das tentativas e devolve o último 502', async () => {
    const fetchMock = vi.fn().mockResolvedValue(res(502))
    vi.stubGlobal('fetch', fetchMock)
    expect((await fetchWithRetry('/api/x', { method: 'PUT' }, [0, 0])).status).toBe(502)
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })
})
