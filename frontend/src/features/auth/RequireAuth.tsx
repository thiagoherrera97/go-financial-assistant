import { Navigate, Outlet } from 'react-router'
import { Layout } from '../../shell/Layout'
import { isSetupRequired } from '../../shared/api/request'
import { setCurrentCycle } from '../../shared/lib/months'
import { useMe } from './api'
import { needsLogin } from './lib/login'

export function RequireAuth() {
  const me = useMe()
  if (me.isPending) return <p className="state">Carregando…</p>
  // App ainda não configurado: a API só atende o setup.
  if (isSetupRequired(me.error)) return <Navigate to="/setup" replace />
  if (needsLogin(me.error)) return <Navigate to="/login" replace />
  // Qualquer outra falha (rede, app reiniciando) não é falta de senha: avisa e deixa tentar de novo.
  if (me.isError)
    return (
      <div className="state" role="alert">
        Não foi possível falar com o FinAssist agora.
        <button type="button" className="btn" onClick={() => void me.refetch()}>
          Tentar de novo
        </button>
      </div>
    )
  setCurrentCycle(me.data.cycle?.month ?? null)
  return (
    <Layout>
      <Outlet />
    </Layout>
  )
}
