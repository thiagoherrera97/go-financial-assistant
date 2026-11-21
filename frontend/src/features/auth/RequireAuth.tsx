import { Navigate, Outlet } from 'react-router'
import { Layout } from '../../shell/Layout'
import { isSetupRequired } from '../../shared/api/request'
import { setCurrentCycle } from '../../shared/lib/months'
import { useMe } from './api'

export function RequireAuth() {
  const me = useMe()
  if (me.isPending) return <p className="state">Carregando…</p>
  // App ainda não configurado: a API só atende o setup.
  if (isSetupRequired(me.error)) return <Navigate to="/setup" replace />
  if (me.isError) return <Navigate to="/login" replace />
  setCurrentCycle(me.data.cycle?.month ?? null)
  return (
    <Layout>
      <Outlet />
    </Layout>
  )
}
