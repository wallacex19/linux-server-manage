import { useState } from 'react'
import { AuthProvider, useAuth } from './context/AuthContext'
import { ServerProvider } from './context/ServerContext'
import { Sidebar, MENU_GROUPS } from './components/layout/Sidebar'
import { Header } from './components/layout/Header'
import { LoginView } from './components/auth/LoginView'
import { DashboardView } from './views/DashboardView'
import { MonitoringView } from './views/MonitoringView'
import { FileManagerView } from './views/FileManagerView'
import { TerminalView } from './views/TerminalView'
import { ProcessesView } from './views/ProcessesView'
import { ServicesView } from './views/ServicesView'
import { SteamCMDView } from './views/SteamCMDView'
import { GameServersView } from './views/GameServersView'
import { ServersView } from './views/ServersView'
import { PlaceholderView } from './views/PlaceholderView'
import { PopularGamePreset } from './types/steamcmd'

function AppContent() {
  const { user, isLoading } = useAuth()
  const [currentView, setCurrentView] = useState('dashboard')
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [selectedPreset, setSelectedPreset] = useState<PopularGamePreset | null>(null)

  if (isLoading) {
    return (
      <div className="min-h-screen bg-neutral-950 flex items-center justify-center text-neutral-400 text-sm">
        <div className="flex items-center gap-3">
          <div className="w-5 h-5 border-2 border-brand-cyan border-t-transparent rounded-full animate-spin" />
          <span>Verificando sessão Linux PAM...</span>
        </div>
      </div>
    )
  }

  if (!user) {
    return <LoginView />
  }

  // Encontra item e categoria ativos para o Header e views
  let activeItemLabel = 'Dashboard'
  let activeCategory = 'Visão geral'

  for (const group of MENU_GROUPS) {
    const found = group.items.find((item) => item.id === currentView)
    if (found) {
      activeItemLabel = found.label
      activeCategory = group.title
      break
    }
  }

  const handleCreateFromPreset = (preset?: PopularGamePreset) => {
    setSelectedPreset(preset || null)
    setCurrentView('game_servers')
  }

  return (
    <div className="flex h-screen bg-neutral-950 text-neutral-100 overflow-hidden font-sans">
      {/* Sidebar Navigation */}
      <Sidebar
        currentView={currentView}
        onSelectView={setCurrentView}
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
      />

      {/* Main Layout Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <Header currentViewLabel={activeItemLabel} onNavigateToServers={() => setCurrentView('servers')} />

        <main className="flex-1 overflow-y-auto p-6 scrollbar-thin">
          <div className="max-w-7xl mx-auto">
            {currentView === 'dashboard' ? (
              <DashboardView onNavigate={setCurrentView} />
            ) : currentView === 'servers' ? (
              <ServersView
                onOpenDashboard={() => setCurrentView('dashboard')}
                onOpenTerminal={() => setCurrentView('terminal')}
              />
            ) : currentView === 'monitoring' ? (
              <MonitoringView />
            ) : currentView === 'filemanager' || currentView === 'recent_files' || currentView === 'transfers' ? (
              <FileManagerView />
            ) : currentView === 'terminal' || currentView === 'persistent_sessions' ? (
              <TerminalView />
            ) : currentView === 'processes' ? (
              <ProcessesView />
            ) : currentView === 'services' ? (
              <ServicesView />
            ) : currentView === 'steamcmd' ? (
              <SteamCMDView onNavigateToCreate={handleCreateFromPreset} />
            ) : currentView === 'game_servers' || currentView === 'game_installs' || currentView === 'server_logs' ? (
              <GameServersView
                onOpenTerminal={() => setCurrentView('terminal')}
                initialPreset={selectedPreset}
              />
            ) : (
              <PlaceholderView
                title={activeItemLabel}
                category={activeCategory}
                roadmapStage={getRoadmapStage(currentView)}
                description={getModuleDescription(currentView)}
              />
            )}
          </div>
        </main>
      </div>
    </div>
  )
}

function getRoadmapStage(viewId: string): string {
  switch (viewId) {
    case 'monitoring':
      return 'Etapa 2 — Dashboard & Métricas'
    case 'filemanager':
    case 'recent_files':
    case 'transfers':
      return 'Etapa 3 — Gerenciador de Arquivos'
    case 'terminal':
    case 'persistent_sessions':
    case 'session_history':
      return 'Etapa 4 — Terminal Interativo & tmux'
    case 'processes':
    case 'services':
      return 'Etapa 5 — Processos & systemd'
    case 'game_servers':
    case 'steamcmd':
    case 'game_installs':
    case 'server_logs':
      return 'Etapa 6 — SteamCMD & Game Servers'
    default:
      return 'Etapa 7 — Estabilização & Configurações'
  }
}

function getModuleDescription(viewId: string): string {
  switch (viewId) {
    case 'users':
      return 'Auditoria de contas Linux locais e grupos do host.'
    case 'storage':
      return 'Monitoramento e montagem de discos e partições de blocos do servidor.'
    case 'network':
      return 'Tabelas de rotas, interfaces de rede e portas abertas do Linux.'
    case 'settings_security':
    case 'settings_panel':
    case 'settings_system':
    case 'settings_ui':
      return 'Configurações de interface, auditoria de segurança e diagnóstico do host.'
    default:
      return 'Módulo planejado e integrado à arquitetura do Linux Server Manager.'
  }
}

export default function App() {
  return (
    <AuthProvider>
      <ServerProvider>
        <AppContent />
      </ServerProvider>
    </AuthProvider>
  )
}
