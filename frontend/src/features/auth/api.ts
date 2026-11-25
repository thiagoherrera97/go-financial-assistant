import { useQuery } from '@tanstack/react-query'
import { postJSON, request } from '../../shared/api/request'
import { retryMe } from './lib/login'

/** Com o segundo fator ativo, a senha certa não abre a sessão: o código vai ao chat ({ step: 'code' }). */
export interface LoginResult {
  ok?: boolean
  step?: 'code'
}

export const login = (password: string) => postJSON<LoginResult>('/api/login', { password })

export const loginCode = (code: string) => postJSON<LoginResult>('/api/login/code', { code })

export const logout = () => postJSON('/api/logout')

export interface Me {
  ok: boolean
  secondFactor: boolean
  /** Login desligado (DASHBOARD_AUTH=off): não há o que sair. */
  noAuth?: boolean
  /** Ciclo do salário de hoje: o "mês atual" das telas. */
  cycle?: { month: string; startsOn: string; endsOn: string }
}

// Sem retry em 401/403 (repetir não muda o resultado); falha de rede e 5xx tentam de novo (retryMe).
export const useMe = () =>
  useQuery({ queryKey: ['me'], queryFn: () => request<Me>('/api/me'), retry: retryMe, staleTime: 60_000 })
