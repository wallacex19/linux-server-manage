import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Gamepad2,
  Plus,
  Play,
  Square,
  RotateCw,
  Trash2,
  Terminal,
  Folder,
  AlertCircle,
  CheckCircle2,
  X,
  RefreshCw,
  Server,
  Layers,
} from 'lucide-react'
import { GameServer, CreateServerRequest, PopularGamePreset } from '../types/steamcmd'
import { useAuth } from '../context/AuthContext'

async function fetchServers(): Promise<GameServer[]> {
  const res = await fetch('/api/v1/games/servers', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao listar servidores de jogos')
  return res.json()
}

async function fetchPresets(): Promise<PopularGamePreset[]> {
  const res = await fetch('/api/v1/steamcmd/presets', { credentials: 'include' })
  if (!res.ok) return []
  return res.json()
}

interface GameServersViewProps {
  onOpenTerminal?: (sessionName?: string) => void
  initialPreset?: PopularGamePreset | null
}

export function GameServersView({ onOpenTerminal, initialPreset }: GameServersViewProps) {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [isModalOpen, setIsModalOpen] = useState(!!initialPreset)
  const [actionMsg, setActionMsg] = useState<{ text: string; isError: boolean } | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  // Formulário de criação
  const [formData, setFormData] = useState<CreateServerRequest>({
    name: initialPreset ? `Servidor ${initialPreset.name}` : '',
    service_type: initialPreset ? initialPreset.service_type : 'custom',
    app_id: initialPreset ? initialPreset.app_id : '',
    install_path: '',
    start_command: initialPreset ? initialPreset.default_args : '',
    port: initialPreset ? initialPreset.default_port : 27015,
    auto_restart: false,
  })

  const { data: servers = [], isLoading } = useQuery({
    queryKey: ['game-servers'],
    queryFn: fetchServers,
    refetchInterval: 5000,
  })

  const { data: presets = [] } = useQuery({
    queryKey: ['steamcmd-presets'],
    queryFn: fetchPresets,
  })

  const refreshServers = () => {
    queryClient.invalidateQueries({ queryKey: ['game-servers'] })
  }

  const handleSelectPreset = (preset: PopularGamePreset) => {
    setFormData({
      name: `Servidor ${preset.name}`,
      service_type: preset.service_type,
      app_id: preset.app_id,
      install_path: `/home/${user?.username || 'user'}/gameservers/${preset.service_type}`,
      start_command: preset.default_args,
      port: preset.default_port,
      auto_restart: false,
    })
  }

  const handleCreateServer = async (e: React.FormEvent) => {
    e.preventDefault()
    setActionMsg(null)

    try {
      const res = await fetch('/api/v1/games/servers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(formData),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Erro ao criar servidor')
      }

      setActionMsg({ text: `Servidor "${formData.name}" cadastrado com sucesso!`, isError: false })
      setIsModalOpen(false)
      refreshServers()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na requisição'
      setActionMsg({ text: msg, isError: true })
    }
  }

  const handleStart = async (server: GameServer) => {
    setActionLoading(`${server.id}-start`)
    setActionMsg(null)
    try {
      const res = await fetch(`/api/v1/games/servers/${server.id}/start`, {
        method: 'POST',
        credentials: 'include',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao iniciar servidor')

      setActionMsg({ text: `Servidor "${server.name}" iniciado em sessão tmux persistente!`, isError: false })
      refreshServers()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na requisição'
      setActionMsg({ text: msg, isError: true })
    } finally {
      setActionLoading(null)
    }
  }

  const handleStop = async (server: GameServer) => {
    if (!window.confirm(`Deseja parar o servidor "${server.name}"?`)) return

    setActionLoading(`${server.id}-stop`)
    setActionMsg(null)
    try {
      const res = await fetch(`/api/v1/games/servers/${server.id}/stop`, {
        method: 'POST',
        credentials: 'include',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao parar servidor')

      setActionMsg({ text: `Servidor "${server.name}" interrompido.`, isError: false })
      refreshServers()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na requisição'
      setActionMsg({ text: msg, isError: true })
    } finally {
      setActionLoading(null)
    }
  }

  const handleUpdate = async (server: GameServer) => {
    if (!window.confirm(`Disparar instalação/atualização via SteamCMD para "${server.name}"?`)) return

    setActionLoading(`${server.id}-update`)
    setActionMsg(null)
    try {
      const res = await fetch(`/api/v1/games/servers/${server.id}/update`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ validate: true }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao acionar atualização')

      setActionMsg({
        text: `Atualização iniciada em segundo plano para "${server.name}". Acompanhe no Terminal!`,
        isError: false,
      })
      refreshServers()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na requisição'
      setActionMsg({ text: msg, isError: true })
    } finally {
      setActionLoading(null)
    }
  }

  const handleDelete = async (server: GameServer) => {
    if (
      !window.confirm(
        `Tem certeza de que deseja remover o servidor "${server.name}" do painel? Os arquivos em disco serão preservados.`,
      )
    ) {
      return
    }

    try {
      const res = await fetch(`/api/v1/games/servers/${server.id}`, {
        method: 'DELETE',
        credentials: 'include',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao excluir servidor')

      setActionMsg({ text: `Servidor "${server.name}" removido com sucesso.`, isError: false })
      refreshServers()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na exclusão'
      setActionMsg({ text: msg, isError: true })
    }
  }

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
            <Gamepad2 className="w-6 h-6 text-brand-cyan" />
            Servidores de Jogos Instalados
          </h1>
          <p className="text-sm text-neutral-400">
            Instâncias dedicadas gerenciadas sob contas de usuário Linux isoladas.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={refreshServers}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-200 text-sm font-medium border border-neutral-700 transition"
          >
            <RefreshCw className="w-4 h-4" />
            Atualizar
          </button>
          <button
            onClick={() => setIsModalOpen(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-brand-cyan hover:bg-cyan-500 text-neutral-950 text-sm font-semibold transition shadow-sm"
          >
            <Plus className="w-4 h-4" />
            Novo Servidor
          </button>
        </div>
      </div>

      {/* Feedback Message */}
      {actionMsg && (
        <div
          className={`p-3 rounded-lg border flex items-center justify-between text-sm ${
            actionMsg.isError
              ? 'bg-rose-500/10 border-rose-500/30 text-rose-300'
              : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
          }`}
        >
          <div className="flex items-center gap-2">
            {actionMsg.isError ? <AlertCircle className="w-4 h-4" /> : <CheckCircle2 className="w-4 h-4" />}
            <span>{actionMsg.text}</span>
          </div>
          <button
            onClick={() => setActionMsg(null)}
            className="text-neutral-400 hover:text-white text-xs underline ml-4"
          >
            Fechar
          </button>
        </div>
      )}

      {/* Servers List */}
      {isLoading ? (
        <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-12 text-center text-neutral-400 text-sm">
          Carregando instâncias de servidores...
        </div>
      ) : servers.length === 0 ? (
        <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-12 text-center">
          <Layers className="w-10 h-10 text-neutral-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-white">Nenhum servidor de jogo cadastrado</h3>
          <p className="text-sm text-neutral-400 mt-1 max-w-md mx-auto">
            Você pode criar um servidor dedicado a partir dos presets pré-configurados (CS2, Valheim, Palworld, TF2) ou configurar um comando personalizado.
          </p>
          <button
            onClick={() => setIsModalOpen(true)}
            className="mt-4 inline-flex items-center gap-1.5 px-4 py-2 rounded bg-brand-cyan text-neutral-950 text-sm font-medium hover:bg-cyan-500 transition"
          >
            <Plus className="w-4 h-4" />
            Criar Primeiro Servidor
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {servers.map((srv) => {
            const isRunning = srv.status === 'running'
            const isUpdating = srv.status === 'updating'

            return (
              <div
                key={srv.id}
                className="bg-neutral-900 border border-neutral-800 rounded-lg p-5 flex flex-col justify-between hover:border-neutral-700 transition"
              >
                <div>
                  <div className="flex items-center justify-between">
                    <span
                      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium ${
                        isRunning
                          ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                          : isUpdating
                          ? 'bg-amber-500/10 text-amber-400 border border-amber-500/30 animate-pulse'
                          : 'bg-neutral-800 text-neutral-400 border border-neutral-700'
                      }`}
                    >
                      <span
                        className={`w-1.5 h-1.5 rounded-full ${
                          isRunning ? 'bg-emerald-400' : isUpdating ? 'bg-amber-400' : 'bg-neutral-500'
                        }`}
                      />
                      {isRunning ? 'Em Execução' : isUpdating ? 'Atualizando Steam...' : 'Parado'}
                    </span>

                    <span className="text-[10px] font-mono text-neutral-500">
                      Porta: {srv.port || 'Auto'}
                    </span>
                  </div>

                  <h3 className="text-base font-bold text-white mt-2 truncate" title={srv.name}>
                    {srv.name}
                  </h3>

                  <div className="mt-3 space-y-1.5 text-xs text-neutral-400">
                    <div className="flex items-center gap-1.5 truncate">
                      <Folder className="w-3.5 h-3.5 text-neutral-500 shrink-0" />
                      <span className="font-mono text-[11px] truncate" title={srv.install_path}>
                        {srv.install_path}
                      </span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <Server className="w-3.5 h-3.5 text-neutral-500 shrink-0" />
                      <span>Usuário Linux: <strong className="text-neutral-200">{srv.system_user}</strong></span>
                    </div>
                    <div className="flex items-center gap-1.5 font-mono text-[11px] text-neutral-500">
                      <span>tmux: {srv.session_name}</span>
                    </div>
                  </div>
                </div>

                <div className="mt-5 pt-4 border-t border-neutral-800 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5">
                    {isRunning ? (
                      <button
                        onClick={() => handleStop(srv)}
                        disabled={actionLoading === `${srv.id}-stop`}
                        title="Parar Servidor"
                        className="p-1.5 rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 text-xs transition"
                      >
                        <Square className="w-4 h-4" />
                      </button>
                    ) : (
                      <button
                        onClick={() => handleStart(srv)}
                        disabled={actionLoading === `${srv.id}-start` || isUpdating}
                        title="Iniciar Servidor"
                        className="p-1.5 rounded bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 text-xs transition disabled:opacity-50"
                      >
                        <Play className="w-4 h-4" />
                      </button>
                    )}

                    <button
                      onClick={() => handleUpdate(srv)}
                      disabled={actionLoading === `${srv.id}-update` || isUpdating}
                      title="Instalar / Atualizar via SteamCMD"
                      className="p-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs transition disabled:opacity-50"
                    >
                      <RotateCw
                        className={`w-4 h-4 ${actionLoading === `${srv.id}-update` ? 'animate-spin' : ''}`}
                      />
                    </button>

                    {onOpenTerminal && (
                      <button
                        onClick={() => onOpenTerminal(srv.session_name)}
                        title="Abrir Console Interativo no Terminal"
                        className="p-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs transition"
                      >
                        <Terminal className="w-4 h-4 text-brand-cyan" />
                      </button>
                    )}
                  </div>

                  <button
                    onClick={() => handleDelete(srv)}
                    title="Remover Servidor do Painel"
                    className="p-1.5 rounded hover:bg-rose-500/20 text-neutral-500 hover:text-rose-400 text-xs transition"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Modal Criar Servidor */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
          <div className="bg-neutral-900 border border-neutral-800 rounded-lg max-w-xl w-full flex flex-col max-h-[90vh] shadow-2xl">
            <div className="flex items-center justify-between px-5 py-4 border-b border-neutral-800">
              <div className="flex items-center gap-2">
                <Gamepad2 className="w-5 h-5 text-brand-cyan" />
                <h3 className="text-base font-semibold text-white">Adicionar Servidor de Jogo</h3>
              </div>
              <button onClick={() => setIsModalOpen(false)} className="text-neutral-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleCreateServer} className="p-5 space-y-4 overflow-y-auto">
              {/* Presets Selector */}
              {presets.length > 0 && (
                <div>
                  <label className="block text-xs font-semibold text-neutral-400 uppercase tracking-wider mb-2">
                    Preencher a partir de Preset Popular
                  </label>
                  <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                    {presets.map((p) => (
                      <button
                        key={p.app_id}
                        type="button"
                        onClick={() => handleSelectPreset(p)}
                        className={`px-2.5 py-1.5 rounded text-xs border text-left truncate transition ${
                          formData.app_id === p.app_id
                            ? 'bg-brand-cyan/20 border-brand-cyan text-brand-cyan font-medium'
                            : 'bg-neutral-950 border-neutral-800 text-neutral-300 hover:border-neutral-700'
                        }`}
                      >
                        {p.name}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <label className="block text-xs font-medium text-neutral-300 mb-1">Nome do Servidor *</label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Servidor de CS2 da Turma"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 focus:outline-none focus:border-brand-cyan"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1">Steam AppID *</label>
                  <input
                    type="text"
                    required
                    placeholder="Ex: 730"
                    value={formData.app_id}
                    onChange={(e) => setFormData({ ...formData, app_id: e.target.value })}
                    className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 font-mono focus:outline-none focus:border-brand-cyan"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1">Porta Principal</label>
                  <input
                    type="number"
                    value={formData.port}
                    onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) || 0 })}
                    className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 font-mono focus:outline-none focus:border-brand-cyan"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-300 mb-1">
                  Diretório de Instalação (Deixe vazio para padrão)
                </label>
                <input
                  type="text"
                  placeholder={`/home/${user?.username || 'user'}/gameservers/...`}
                  value={formData.install_path}
                  onChange={(e) => setFormData({ ...formData, install_path: e.target.value })}
                  className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 font-mono focus:outline-none focus:border-brand-cyan"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-neutral-300 mb-1">Comando de Inicialização</label>
                <textarea
                  rows={2}
                  placeholder="./start.sh"
                  value={formData.start_command}
                  onChange={(e) => setFormData({ ...formData, start_command: e.target.value })}
                  className="w-full bg-neutral-950 border border-neutral-800 text-xs text-neutral-200 rounded px-3 py-2 font-mono focus:outline-none focus:border-brand-cyan"
                />
              </div>

              <div className="flex items-center gap-2 pt-2">
                <input
                  type="checkbox"
                  id="auto_restart"
                  checked={formData.auto_restart}
                  onChange={(e) => setFormData({ ...formData, auto_restart: e.target.checked })}
                  className="rounded bg-neutral-950 border-neutral-800 text-brand-cyan focus:ring-0"
                />
                <label htmlFor="auto_restart" className="text-xs text-neutral-300 select-none cursor-pointer">
                  Reiniciar automaticamente em caso de encerramento inesperado
                </label>
              </div>

              <div className="flex items-center justify-end gap-2 pt-4 border-t border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-4 py-2 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs font-medium"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded bg-brand-cyan hover:bg-cyan-500 text-neutral-950 text-xs font-bold transition"
                >
                  Salvar Servidor
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
