import { describe, expect, it } from 'vitest'
import { loginErrorMessage, needsLogin, retryMe } from './login'
import { ApiError } from '../../../shared/api/request'

describe('loginErrorMessage', () => {
  it('sem erro, sem mensagem', () => {
    expect(loginErrorMessage(null, 'password')).toBeNull()
  })
  it('senha errada e limite por IP', () => {
    expect(loginErrorMessage(new ApiError(401, 'senha incorreta'), 'password')).toBe('Senha incorreta.')
    expect(loginErrorMessage(new ApiError(429, 'erro inesperado'), 'password')).toMatch(/Aguarde um minuto/)
  })
  it('usa a mensagem do servidor no passo do código', () => {
    expect(loginErrorMessage(new ApiError(401, 'código incorreto'), 'code')).toBe('Código incorreto.')
    expect(loginErrorMessage(new ApiError(503, 'não foi possível enviar o código ao chat'), 'password')).toBe('Não foi possível enviar o código ao chat.')
  })
  it('erro de rede ou sem corpo vira a mensagem genérica', () => {
    expect(loginErrorMessage(new TypeError('fetch'), 'code')).toMatch(/Tente de novo/)
    expect(loginErrorMessage(new ApiError(500, 'erro inesperado'), 'code')).toMatch(/Tente de novo/)
  })
})

describe('needsLogin', () => {
  it('só 401 manda ao login', () => {
    expect(needsLogin(new ApiError(401, 'sem sessão'))).toBe(true)
    expect(needsLogin(new ApiError(502, 'erro inesperado'))).toBe(false)
    expect(needsLogin(new TypeError('Failed to fetch'))).toBe(false)
  })
})

describe('retryMe', () => {
  it('repete falha de rede e 5xx, não repete 4xx', () => {
    expect(retryMe(0, new TypeError('Failed to fetch'))).toBe(true)
    expect(retryMe(0, new ApiError(502, 'erro inesperado'))).toBe(true)
    expect(retryMe(3, new ApiError(502, 'erro inesperado'))).toBe(false)
    expect(retryMe(0, new ApiError(401, 'sem sessão'))).toBe(false)
  })
})
