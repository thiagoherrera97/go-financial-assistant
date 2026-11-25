import { ApiError } from '../../../shared/api/request'

/** A mensagem de erro de cada passo do login. As do servidor (código expirado, errado, canal fora) já vêm prontas. */
export function loginErrorMessage(error: unknown, step: 'password' | 'code'): string | null {
  if (!error) return null
  if (!(error instanceof ApiError)) return 'Não foi possível entrar. Tente de novo.'
  if (error.status === 401 && step === 'password') return 'Senha incorreta.'
  // O limite por IP responde sem JSON, então chega com a mensagem genérica.
  if (error.status === 429 && error.message === 'erro inesperado') return 'Muitas tentativas. Aguarde um minuto e tente de novo.'
  if (error.message === 'erro inesperado') return 'Não foi possível entrar. Tente de novo.'
  return error.message.charAt(0).toUpperCase() + error.message.slice(1) + '.'
}

/**
 * Só a API dizendo "sem sessão" (401) leva ao login. Falha de rede, app reiniciando (502) ou o celular voltando
 * do bloqueio com o Tailscale ainda reconectando não são falta de login: mandar para a tela de senha nesses casos
 * deixava preso no login quem nem usa senha (DASHBOARD_AUTH=off).
 */
export const needsLogin = (error: unknown): boolean => error instanceof ApiError && error.status === 401

/** Repete a checagem de sessão em falhas passageiras (até 3 vezes); 4xx é resposta definitiva. */
export const retryMe = (failures: number, error: unknown): boolean =>
  failures < 3 && !(error instanceof ApiError && error.status >= 400 && error.status < 500)
