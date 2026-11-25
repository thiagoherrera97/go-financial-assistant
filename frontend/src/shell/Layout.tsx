import { useQueryClient } from '@tanstack/react-query'
import {
  FileSpreadsheet,
  ArrowLeftRight,
  CreditCard,
  Bot,
  Calculator,
  ChartPie,
  Gem,
  GitCompareArrows,
  Landmark,
  LayoutDashboard,
  LogOut,
  Menu,
  Moon,
  ScanSearch,
  Scale,
  Settings,
  Sun,
  Tags,
  Target,
  TrendingUp,
  X,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router'
import { useTheme } from '../shared/lib/theme'
import { logout, useMe } from '../features/auth/api'
import { SyncControl } from '../shared/components/SyncControl'
import { ComposeFab, ComposePanel } from '../shared/components/ComposePanel'
import { ComposeProvider } from '../shared/lib/compose'

interface Item {
  to: string
  label: string
  Icon: LucideIcon
}

// O menu é agrupado por assunto; a ordem dentro de cada grupo vai do mais usado ao menos usado.
// tone: a cor do quadradinho atrás dos ícones da seção (ajuda a achar o grupo de olho; ver .side-ico).
const SECTIONS: { title?: string; tone: string; items: Item[] }[] = [
  {
    tone: 'home',
    items: [
      { to: '/', label: 'Painel', Icon: Landmark },
      { to: '/mes', label: 'Ciclo', Icon: FileSpreadsheet },
      { to: '/visao-geral', label: 'Visão geral', Icon: LayoutDashboard },
    ],
  },
  {
    title: 'Dia a dia',
    tone: 'daily',
    items: [
      { to: '/transacoes', label: 'Transações', Icon: ArrowLeftRight },
      { to: '/faturas', label: 'Faturas', Icon: CreditCard },
      { to: '/gastos', label: 'Gastos', Icon: ChartPie },
      { to: '/classificar', label: 'Classificar', Icon: Tags },
      { to: '/orcamento', label: 'Orçamento', Icon: Scale },
    ],
  },
  {
    title: 'Planejamento',
    tone: 'plan',
    items: [
      { to: '/revisao', label: 'Revisão', Icon: ScanSearch },
      { to: '/metas', label: 'Metas', Icon: Target },
      { to: '/fornecedores', label: 'Fornecedores', Icon: Gem },
      { to: '/coach', label: 'Coach', Icon: Bot },
      { to: '/projecao', label: 'Projeção', Icon: Calculator },
      { to: '/comparacoes', label: 'Comparações', Icon: GitCompareArrows },
    ],
  },
  { title: 'Patrimônio', tone: 'wealth', items: [{ to: '/investimentos', label: 'Investimentos', Icon: TrendingUp }] },
]

export function Layout({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { theme, toggle } = useTheme()
  const me = useMe().data
  const noAuth = me?.noAuth ?? false
  const owner = me?.name?.trim() ?? ''
  const screen = screenLabel(pathname)

  // A aba do navegador diz a tela e de quem é o app: com duas instalações abertas, dá para saber qual é qual.
  useEffect(() => {
    document.title = [screen, owner ? `FinAssist de ${owner}` : 'FinAssist'].filter(Boolean).join(' · ')
  }, [screen, owner])
  const [open, setOpen] = useState(false)

  // Em telas estreitas o menu é uma gaveta: Esc fecha.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  async function handleLogout() {
    // Se o logout falhar, o cookie expira sozinho; mesmo assim saímos da tela.
    await logout().catch(() => undefined)
    queryClient.clear()
    navigate('/login', { replace: true })
  }

  return (
    <ComposeProvider>
      <div className="app">
        <header className="mobilebar">
          <button type="button" className="icon-btn" aria-label="Abrir menu" aria-expanded={open} aria-controls="sidebar" onClick={() => setOpen(true)}>
            <Menu size={22} aria-hidden="true" />
          </button>
          {/* No celular o topo diz a tela atual e de quem é o app (o título da página some ao rolar). */}
          <span className="mobilebar-title">
            <span className="brand-mark" aria-hidden="true">{owner ? owner.charAt(0).toUpperCase() : 'F'}</span>
            <span className="mobilebar-text">
              <strong>{screen || 'FinAssist'}</strong>
              <small>{owner ? `FinAssist de ${owner}` : 'FinAssist'}</small>
            </span>
          </span>
          <span className="mobilebar-end">
            <SyncControl compact />
          </span>
        </header>

        {open && <div className="scrim" onClick={() => setOpen(false)} aria-hidden="true" />}

        <aside id="sidebar" className={`sidebar ${open ? 'is-open' : ''}`}>
          <div className="side-brand">
            <span className="brand">
              <span className="brand-mark" aria-hidden="true">{owner ? owner.charAt(0).toUpperCase() : 'F'}</span>
              <span className="brand-text">
                FinAssist
                {owner && <small className="brand-owner">de {owner}</small>}
              </span>
            </span>
            <button type="button" className="icon-btn side-close" aria-label="Fechar menu" onClick={() => setOpen(false)}>
              <X size={20} aria-hidden="true" />
            </button>
          </div>

          <SyncControl />

          <nav aria-label="Principal" className="side-nav">
            {SECTIONS.map((section, i) => (
              <div className={`side-section tone-${section.tone}`} key={section.title ?? i}>
                {section.title && <p className="side-title">{section.title}</p>}
                {section.items.map(({ to, label, Icon }) => (
                  <NavLink key={to} to={to} end={to === '/'} className="side-link" onClick={() => setOpen(false)}>
                    <span className="side-ico" aria-hidden="true">
                      <Icon size={17} />
                    </span>
                    {label}
                  </NavLink>
                ))}
              </div>
            ))}
          </nav>

          <div className="side-foot">
            <NavLink to="/configuracoes" className="side-link" onClick={() => setOpen(false)}>
              <Settings size={18} aria-hidden="true" />
              Configurações
            </NavLink>
            <button type="button" className="side-link" onClick={toggle}>
              {theme === 'dark' ? <Sun size={18} aria-hidden="true" /> : <Moon size={18} aria-hidden="true" />}
              {theme === 'dark' ? 'Tema claro' : 'Tema escuro'}
            </button>
            {!noAuth && (
              <button type="button" className="side-link" onClick={handleLogout}>
                <LogOut size={18} aria-hidden="true" />
                Sair
              </button>
            )}
          </div>
        </aside>

        <main className="content">
          {/* A chave remonta o miolo a cada tela: a entrada (.route) só anima na troca de página. */}
          <div className="container route" key={pathname}>
            {children}
          </div>
        </main>

        <ComposeFab />
        <ComposePanel />
      </div>
    </ComposeProvider>
  )
}

/** O nome da tela atual, pelo menu (Configurações fica fora dele). */
function screenLabel(pathname: string): string {
  if (pathname.startsWith('/configuracoes')) return 'Configurações'
  for (const section of SECTIONS) {
    const item = section.items.find(({ to }) => (to === '/' ? pathname === '/' : pathname.startsWith(to)))
    if (item) return item.label
  }
  return ''
}
