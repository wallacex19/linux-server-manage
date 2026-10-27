import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Terminal as TerminalIcon,
  Plus,
  X,
  Radio,
  Trash2,
  RefreshCw,
  Gamepad2,
  Layers,
} from 'lucide-react'
import { XTermTerminal } from '../components/terminal/XTermTerminal'
import { PersistentSession, TerminalTab } from '../types/terminal'

async function fetchTmuxSessions(): Promise<PersistentSession[]> {
  const res = await fetch('/api/v1/terminal/sessions', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao listar sessões tmux')
  return res.json()
}

export function TerminalView() {
  const queryClient = useQueryClient()
  const [tabs, setTabs] = useState<TerminalTab[]>([
    { id: 'tab-1', title: 'Terminal Shell' },
  ])
  const [activeTabId, setActiveTabId] = useState<string>('tab-1')
  const [showSessionsPanel, setShowSessionsPanel] = useState<boolean>(true)

  const { data: sessions = [], isLoading: sessionsLoading } = useQuery({
    queryKey: ['tmux-sessions'],
    queryFn: fetchTmuxSessions,
    refetchInterval: 5000,
  })

  const refreshSessions = () => {
    queryClient.invalidateQueries({ queryKey: ['tmux-sessions'] })
  }

  const handleAddTab = () => {
    const newId = `tab-${Date.now()}`
    const newTab: TerminalTab = {
      id: newId,
      title: `Terminal ${tabs.length + 1}`,
    }
    setTabs([...tabs, newTab])
    setActiveTabId(newId)
  }

  const handleCloseTab = (tabId: string) => {
    if (tabs.length === 1) return // mantém pelo menos 1 aba
    const nextTabs = tabs.filter((t) => t.id !== tabId)
    setTabs(nextTabs)
    if (activeTabId === tabId) {
      setActiveTabId(nextTabs[nextTabs.length - 1].id)
    }
  }

  const handleConnectToSession = (session: PersistentSession) => {
    // Verifica se já existe aba aberta para esta sessão
    const existing = tabs.find((t) => t.tmuxSession === session.name)
    if (existing) {
      setActiveTabId(existing.id)
      return
    }

    const newId = `tab-tmux-${session.name}`
    const newTab: TerminalTab = {
      id: newId,
      title: `tmux: ${session.name}`,
      tmuxSession: session.name,
    }
    setTabs([...tabs, newTab])
    setActiveTabId(newId)
  }

  const handleCreateSession = async () => {
    const name = window.prompt('Nome da nova sessão persistente (ex: game-rust):')
    if (!name || !name.trim()) return

    try {
      const res = await fetch('/api/v1/terminal/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name: name.trim() }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao criar sessão tmux')
      }
      refreshSessions()
      const newSession: PersistentSession = await res.json()
      handleConnectToSession(newSession)
    } catch (err: unknown) {
      window.alert(err instanceof Error ? err.message : 'Erro ao criar sessão')
    }
  }

  const handleKillSession = async (sessionName: string) => {
    if (
      !window.confirm(
        `Tem certeza que deseja encerrar permanentemente a sessão tmux "${sessionName}"? Todos os processos em execução nela serão finalizados.`,
      )
    ) {
      return
    }

    try {
      const res = await fetch(
        `/api/v1/terminal/sessions?name=${encodeURIComponent(sessionName)}`,
        {
          method: 'DELETE',
          credentials: 'include',
        },
      )
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao encerrar sessão')
      }
      refreshSessions()
      // Fecha a aba se estiver aberta
      const tabToClose = tabs.find((t) => t.tmuxSession === sessionName)
      if (tabToClose) {
        handleCloseTab(tabToClose.id)
      }
    } catch (err: unknown) {
      window.alert(err instanceof Error ? err.message : 'Erro ao encerrar sessão')
    }
  }

  const activeTab = tabs.find((t) => t.id === activeTabId)

  return (
    <div className="flex flex-col h-[calc(100vh-6.5rem)] space-y-3">
      {/* Top Controls & Tab Bar */}
      <div className="flex items-center justify-between gap-4 bg-slate-900 border border-slate-800 rounded-xl px-4 py-2 shrink-0">
        {/* Tabs Bar */}
        <div className="flex items-center gap-1.5 overflow-x-auto scrollbar-thin">
          {tabs.map((tab) => {
            const isActive = tab.id === activeTabId
            return (
              <div
                key={tab.id}
                onClick={() => setActiveTabId(tab.id)}
                className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium cursor-pointer transition-colors border select-none ${
                  isActive
                    ? 'bg-slate-950 text-white border-slate-700 shadow-sm'
                    : 'bg-slate-900/60 text-slate-400 hover:text-slate-200 border-transparent hover:bg-slate-800'
                }`}
              >
                <TerminalIcon className={`w-3.5 h-3.5 ${isActive ? 'text-indigo-400' : 'text-slate-500'}`} />
                <span className="truncate max-w-[120px]">{tab.title}</span>
                {tabs.length > 1 && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      handleCloseTab(tab.id)
                    }}
                    className="p-0.5 rounded hover:bg-slate-800 text-slate-500 hover:text-white transition-colors"
                  >
                    <X className="w-3 h-3" />
                  </button>
                )}
              </div>
            )
          })}
          <button
            onClick={handleAddTab}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
            title="Abrir nova aba de terminal"
          >
            <Plus className="w-4 h-4" />
          </button>
        </div>

        {/* Persistent Sessions Toggle */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={() => setShowSessionsPanel(!showSessionsPanel)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
              showSessionsPanel
                ? 'bg-indigo-600 text-white border-indigo-500'
                : 'bg-slate-800 text-slate-300 border-slate-700 hover:text-white'
            }`}
          >
            <Radio className="w-3.5 h-3.5" />
            <span>Sessões tmux ({sessions.length})</span>
          </button>
        </div>
      </div>

      {/* Main Terminal Area & Persistent Sessions Drawer */}
      <div className="flex-1 flex gap-4 min-h-0 overflow-hidden">
        {/* Terminal Screen Container */}
        <div className="flex-1 h-full min-h-0 min-w-0">
          {activeTab && (
            <XTermTerminal
              key={activeTab.id}
              tmuxSession={activeTab.tmuxSession}
            />
          )}
        </div>

        {/* Persistent tmux Sessions Sidebar Panel */}
        {showSessionsPanel && (
          <div className="w-80 bg-slate-900 border border-slate-800 rounded-xl p-4 flex flex-col shrink-0 shadow-lg overflow-hidden">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800 shrink-0">
              <div className="flex items-center gap-2">
                <Radio className="w-4 h-4 text-indigo-400" />
                <h3 className="text-xs font-semibold text-white">Sessões Persistentes</h3>
              </div>
              <div className="flex items-center gap-1">
                <button
                  onClick={refreshSessions}
                  className="p-1 text-slate-400 hover:text-white rounded hover:bg-slate-800"
                  title="Atualizar sessões"
                >
                  <RefreshCw className="w-3 h-3" />
                </button>
                <button
                  onClick={() => setShowSessionsPanel(false)}
                  className="p-1 text-slate-400 hover:text-white rounded hover:bg-slate-800"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            </div>

            <p className="text-[11px] text-slate-400 py-2 leading-relaxed shrink-0">
              Processos em sessões tmux continuam rodando em segundo plano mesmo após fechar o navegador.
            </p>

            <button
              onClick={handleCreateSession}
              className="w-full mb-3 py-1.5 px-3 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-medium flex items-center justify-center gap-1.5 transition-colors shrink-0"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>Nova Sessão tmux</span>
            </button>

            {/* List of Sessions */}
            <div className="flex-1 overflow-y-auto space-y-2 scrollbar-thin pr-1">
              {sessionsLoading ? (
                <div className="text-center py-6 text-xs text-slate-500">
                  Carregando sessões...
                </div>
              ) : sessions.length === 0 ? (
                <div className="text-center py-8 text-xs text-slate-500">
                  Nenhuma sessão tmux ativa.
                </div>
              ) : (
                sessions.map((sess) => (
                  <div
                    key={sess.name}
                    className="p-3 bg-slate-950/70 border border-slate-800 rounded-lg space-y-2 text-xs"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-1.5 font-medium text-white truncate">
                        {sess.is_game_server ? (
                          <Gamepad2 className="w-3.5 h-3.5 text-purple-400 shrink-0" />
                        ) : (
                          <Layers className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
                        )}
                        <span className="truncate">{sess.name}</span>
                      </div>
                      <button
                        onClick={() => handleKillSession(sess.name)}
                        className="p-1 rounded text-slate-500 hover:text-rose-400 hover:bg-rose-950/40 transition-colors"
                        title="Encerrar sessão tmux"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>

                    <div className="flex items-center justify-between text-[11px] text-slate-400">
                      <span>Janelas: {sess.windows_count}</span>
                      <span className={sess.is_attached ? 'text-emerald-400' : 'text-slate-500'}>
                        {sess.is_attached ? 'Conectada' : 'Desconectada'}
                      </span>
                    </div>

                    <button
                      onClick={() => handleConnectToSession(sess)}
                      className="w-full py-1 rounded bg-indigo-600/20 hover:bg-indigo-600/40 text-indigo-300 border border-indigo-500/30 text-xs font-medium transition-colors"
                    >
                      Conectar ao Console
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
