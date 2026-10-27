import {
  LayoutDashboard,
  Activity,
  Bell,
  Cpu,
  Layers,
  Users,
  HardDrive,
  Network,
  Folder,
  Clock,
  ArrowLeftRight,
  Terminal,
  Radio,
  History,
  Gamepad2,
  DownloadCloud,
  RefreshCw,
  FileText,
  Sliders,
  Shield,
  Settings,
  Info,
  ChevronLeft,
  ChevronRight,
  Server,
} from 'lucide-react'

export interface MenuItem {
  id: string
  label: string
  icon: typeof LayoutDashboard
}

export interface MenuGroup {
  title: string
  items: MenuItem[]
}

export const MENU_GROUPS: MenuGroup[] = [
  {
    title: 'Visão geral',
    items: [
      { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
      { id: 'servers', label: 'Servidores', icon: Server },
      { id: 'monitoring', label: 'Monitoramento', icon: Activity },
      { id: 'alerts', label: 'Alertas', icon: Bell },
    ],
  },
  {
    title: 'Sistema',
    items: [
      { id: 'processes', label: 'Processos', icon: Cpu },
      { id: 'services', label: 'Serviços do Linux', icon: Layers },
      { id: 'users', label: 'Usuários e permissões', icon: Users },
      { id: 'storage', label: 'Armazenamento', icon: HardDrive },
      { id: 'network', label: 'Rede', icon: Network },
    ],
  },
  {
    title: 'Arquivos',
    items: [
      { id: 'filemanager', label: 'Gerenciador de arquivos', icon: Folder },
      { id: 'recent_files', label: 'Arquivos recentes', icon: Clock },
      { id: 'transfers', label: 'Transferências', icon: ArrowLeftRight },
    ],
  },
  {
    title: 'Terminal',
    items: [
      { id: 'terminal', label: 'Terminal interativo', icon: Terminal },
      { id: 'persistent_sessions', label: 'Sessões persistentes', icon: Radio },
      { id: 'session_history', label: 'Histórico de sessões', icon: History },
    ],
  },
  {
    title: 'Servidores de jogos',
    items: [
      { id: 'game_servers', label: 'Servidores instalados', icon: Gamepad2 },
      { id: 'steamcmd', label: 'SteamCMD', icon: DownloadCloud },
      { id: 'game_installs', label: 'Instalações e atualizações', icon: RefreshCw },
      { id: 'server_logs', label: 'Logs dos servidores', icon: FileText },
    ],
  },
  {
    title: 'Configurações',
    items: [
      { id: 'ui_preferences', label: 'Preferências da interface', icon: Sliders },
      { id: 'security_settings', label: 'Segurança', icon: Shield },
      { id: 'panel_config', label: 'Configurações do painel', icon: Settings },
      { id: 'system_info', label: 'Informações do sistema', icon: Info },
    ],
  },
]

interface SidebarProps {
  currentView: string
  onSelectView: (viewId: string) => void
  collapsed: boolean
  onToggleCollapse: () => void
}

export function Sidebar({
  currentView,
  onSelectView,
  collapsed,
  onToggleCollapse,
}: SidebarProps) {
  return (
    <aside
      className={`relative flex flex-col bg-slate-900 border-r border-slate-800 transition-all duration-300 select-none ${
        collapsed ? 'w-16' : 'w-64'
      }`}
    >
      {/* Brand Header */}
      <div className="flex items-center justify-between px-4 h-16 border-b border-slate-800">
        <div className="flex items-center gap-3 overflow-hidden">
          <div className="p-2 bg-indigo-500/10 border border-indigo-500/20 rounded-lg text-indigo-400 shrink-0">
            <Server className="w-5 h-5" />
          </div>
          {!collapsed && (
            <div className="truncate">
              <span className="font-semibold text-sm text-white tracking-tight">LSM Manager</span>
              <p className="text-[10px] text-slate-400 truncate">Linux Host & Games</p>
            </div>
          )}
        </div>
        <button
          onClick={onToggleCollapse}
          className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
          title={collapsed ? 'Expandir menu lateral' : 'Recolher menu lateral'}
        >
          {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
        </button>
      </div>

      {/* Navigation Groups */}
      <div className="flex-1 overflow-y-auto py-3 px-2 space-y-4 scrollbar-thin">
        {MENU_GROUPS.map((group) => (
          <div key={group.title} className="space-y-1">
            {!collapsed && (
              <h2 className="px-3 text-[11px] font-medium uppercase tracking-wider text-slate-500">
                {group.title}
              </h2>
            )}
            <ul className="space-y-0.5">
              {group.items.map((item) => {
                const Icon = item.icon
                const isActive = currentView === item.id
                return (
                  <li key={item.id}>
                    <button
                      onClick={() => onSelectView(item.id)}
                      title={collapsed ? item.label : undefined}
                      className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-medium transition-colors ${
                        isActive
                          ? 'bg-indigo-600 text-white font-semibold shadow-sm'
                          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                      } ${collapsed ? 'justify-center px-0' : ''}`}
                    >
                      <Icon className={`w-4 h-4 shrink-0 ${isActive ? 'text-white' : 'text-slate-400'}`} />
                      {!collapsed && <span className="truncate">{item.label}</span>}
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </div>
    </aside>
  )
}
