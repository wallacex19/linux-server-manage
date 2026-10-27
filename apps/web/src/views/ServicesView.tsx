import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Server,
  Play,
  Square,
  RotateCw,
  Search,
  FileText,
  AlertTriangle,
  CheckCircle2,
  AlertCircle,
  X,
  ShieldAlert,
} from 'lucide-react'
import { ServiceInfo } from '../types/system'
import { useAuth } from '../context/AuthContext'

async function fetchServices(): Promise<ServiceInfo[]> {
  const res = await fetch('/api/v1/system/services', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao listar serviços do systemd')
  return res.json()
}

export function ServicesView() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [searchTerm, setSearchTerm] = useState('')
  const [stateFilter, setStateFilter] = useState<'all' | 'active' | 'inactive' | 'failed'>('all')
  const [selectedLogService, setSelectedLogService] = useState<string | null>(null)
  const [logContent, setLogContent] = useState<string>('')
  const [logLoading, setLogLoading] = useState<boolean>(false)
  const [actionMsg, setActionMsg] = useState<{ text: string; isError: boolean } | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  const { data: services = [], isLoading, error } = useQuery({
    queryKey: ['system-services'],
    queryFn: fetchServices,
    refetchInterval: 10000,
  })

  const refreshServices = () => {
    queryClient.invalidateQueries({ queryKey: ['system-services'] })
  }

  const handleAction = async (serviceName: string, action: 'start' | 'stop' | 'restart') => {
    const actionLabel = action === 'start' ? 'iniciar' : action === 'stop' ? 'parar' : 'reiniciar'
    if (!window.confirm(`Tem certeza que deseja ${actionLabel} o serviço "${serviceName}"?`)) {
      return
    }

    setActionLoading(`${serviceName}-${action}`)
    setActionMsg(null)

    try {
      const res = await fetch('/api/v1/system/services/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name: serviceName, action }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Erro ao executar ação no serviço')
      }

      setActionMsg({ text: `Serviço "${serviceName}": ${action} executado com sucesso`, isError: false })
      refreshServices()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na requisição'
      setActionMsg({ text: msg, isError: true })
    } finally {
      setActionLoading(null)
    }
  }

  const handleOpenLogs = async (serviceName: string) => {
    setSelectedLogService(serviceName)
    setLogLoading(true)
    setLogContent('')

    try {
      const res = await fetch(`/api/v1/system/services/logs?name=${encodeURIComponent(serviceName)}&lines=100`, {
        credentials: 'include',
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Erro ao carregar logs do serviço')
      }
      setLogContent(data.logs || 'Nenhum log encontrado para este serviço.')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Erro ao buscar logs'
      setLogContent(`[ERRO]: ${msg}`)
    } finally {
      setLogLoading(false)
    }
  }

  const filteredServices = services.filter((svc) => {
    const matchesSearch =
      svc.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      svc.description.toLowerCase().includes(searchTerm.toLowerCase())

    if (!matchesSearch) return false

    if (stateFilter === 'active') return svc.active_state === 'active'
    if (stateFilter === 'inactive') return svc.active_state === 'inactive'
    if (stateFilter === 'failed') return svc.active_state === 'failed'
    return true
  })

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
            <Server className="w-6 h-6 text-brand-cyan" />
            Serviços do Linux (systemd)
          </h1>
          <p className="text-sm text-neutral-400">
            Gerenciamento e inspeção de unidades de serviço systemd no host Linux.
          </p>
        </div>

        <button
          onClick={refreshServices}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-200 text-sm font-medium border border-neutral-700 transition"
        >
          <RotateCw className="w-4 h-4" />
          Atualizar
        </button>
      </div>

      {/* Admin Notice if not admin */}
      {!user?.is_admin && (
        <div className="p-4 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-start gap-3">
          <ShieldAlert className="w-5 h-5 text-amber-400 mt-0.5 shrink-0" />
          <div>
            <h4 className="text-sm font-semibold text-amber-300">Modo de Apenas Leitura</h4>
            <p className="text-xs text-amber-200/80 mt-0.5">
              O usuário <strong>{user?.username}</strong> não possui privilégios de administrador (sudo/wheel). As ações de iniciar, parar e reiniciar serviços do sistema exigem permissões de superusuário ou polkit.
            </p>
          </div>
        </div>
      )}

      {/* Feedback Alert */}
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

      {/* Filter and Search Bar */}
      <div className="flex flex-col sm:flex-row items-center gap-3 bg-neutral-900 border border-neutral-800 p-3 rounded-lg">
        <div className="relative flex-1 w-full">
          <Search className="w-4 h-4 text-neutral-500 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Buscar por nome ou descrição do serviço..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded pl-9 pr-3 py-1.5 focus:outline-none focus:border-brand-cyan"
          />
        </div>

        <div className="flex items-center gap-2 w-full sm:w-auto">
          <span className="text-xs text-neutral-400">Estado:</span>
          <select
            value={stateFilter}
            onChange={(e) => setStateFilter(e.target.value as 'all' | 'active' | 'inactive' | 'failed')}
            className="bg-neutral-950 border border-neutral-800 text-xs text-neutral-200 rounded px-2.5 py-1.5 focus:outline-none focus:border-brand-cyan"
          >
            <option value="all">Todos os Estados</option>
            <option value="active">Ativos (Running)</option>
            <option value="inactive">Inativos</option>
            <option value="failed">Falhas (Failed)</option>
          </select>
        </div>
      </div>

      {/* Services Table */}
      <div className="bg-neutral-900 border border-neutral-800 rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-neutral-400 text-sm">Carregando serviços do systemd...</div>
        ) : error ? (
          <div className="p-8 text-center text-rose-400 text-sm flex items-center justify-center gap-2">
            <AlertTriangle className="w-4 h-4" />
            Falha ao carregar lista de serviços
          </div>
        ) : filteredServices.length === 0 ? (
          <div className="p-8 text-center text-neutral-500 text-sm">
            Nenhum serviço encontrado com os filtros selecionados.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm text-neutral-300">
              <thead className="bg-neutral-950 text-xs uppercase text-neutral-400 border-b border-neutral-800">
                <tr>
                  <th className="px-4 py-3 font-medium">Nome da Unidade</th>
                  <th className="px-4 py-3 font-medium">Descrição</th>
                  <th className="px-4 py-3 font-medium">Estado</th>
                  <th className="px-4 py-3 font-medium">Sub-estado</th>
                  <th className="px-4 py-3 font-medium text-right">Ações</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-800/60">
                {filteredServices.map((svc) => {
                  const isActive = svc.active_state === 'active'
                  const isFailed = svc.active_state === 'failed'
                  const canControl = user?.is_admin

                  return (
                    <tr key={svc.name} className="hover:bg-neutral-800/40 transition">
                      <td className="px-4 py-3 font-mono text-xs text-white font-semibold">
                        {svc.name}
                      </td>
                      <td className="px-4 py-3 text-xs text-neutral-400 max-w-xs truncate" title={svc.description}>
                        {svc.description || '—'}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                            isActive
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                              : isFailed
                              ? 'bg-rose-500/10 text-rose-400 border border-rose-500/30'
                              : 'bg-neutral-800 text-neutral-400 border border-neutral-700'
                          }`}
                        >
                          {svc.active_state}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-xs text-neutral-400 font-mono">
                        {svc.sub_state}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          {/* Logs Button */}
                          <button
                            onClick={() => handleOpenLogs(svc.name)}
                            title="Ver Logs (journalctl)"
                            className="p-1 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs transition"
                          >
                            <FileText className="w-3.5 h-3.5" />
                          </button>

                          {/* Control Buttons */}
                          {isActive ? (
                            <button
                              onClick={() => handleAction(svc.name, 'stop')}
                              disabled={!canControl || actionLoading === `${svc.name}-stop`}
                              title={canControl ? 'Parar Serviço' : 'Requer privilégios de administrador'}
                              className={`p-1 rounded text-xs transition ${
                                canControl
                                  ? 'bg-rose-500/20 text-rose-300 hover:bg-rose-500/30'
                                  : 'bg-neutral-800/50 text-neutral-600 cursor-not-allowed'
                              }`}
                            >
                              <Square className="w-3.5 h-3.5" />
                            </button>
                          ) : (
                            <button
                              onClick={() => handleAction(svc.name, 'start')}
                              disabled={!canControl || actionLoading === `${svc.name}-start`}
                              title={canControl ? 'Iniciar Serviço' : 'Requer privilégios de administrador'}
                              className={`p-1 rounded text-xs transition ${
                                canControl
                                  ? 'bg-emerald-500/20 text-emerald-300 hover:bg-emerald-500/30'
                                  : 'bg-neutral-800/50 text-neutral-600 cursor-not-allowed'
                              }`}
                            >
                              <Play className="w-3.5 h-3.5" />
                            </button>
                          )}

                          <button
                            onClick={() => handleAction(svc.name, 'restart')}
                            disabled={!canControl || actionLoading === `${svc.name}-restart`}
                            title={canControl ? 'Reiniciar Serviço' : 'Requer privilégios de administrador'}
                            className={`p-1 rounded text-xs transition ${
                              canControl
                              ? 'bg-neutral-800 hover:bg-neutral-700 text-neutral-300'
                              : 'bg-neutral-800/50 text-neutral-600 cursor-not-allowed'
                            }`}
                          >
                            <RotateCw
                              className={`w-3.5 h-3.5 ${
                                actionLoading === `${svc.name}-restart` ? 'animate-spin' : ''
                              }`}
                            />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Service Journal Logs Modal */}
      {selectedLogService && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
          <div className="bg-neutral-900 border border-neutral-800 rounded-lg max-w-3xl w-full flex flex-col max-h-[85vh] shadow-2xl">
            <div className="flex items-center justify-between px-4 py-3 border-b border-neutral-800">
              <div className="flex items-center gap-2">
                <FileText className="w-5 h-5 text-brand-cyan" />
                <h3 className="text-base font-semibold text-white">
                  Logs do Serviço: <span className="font-mono text-brand-cyan">{selectedLogService}</span>
                </h3>
              </div>
              <button
                onClick={() => setSelectedLogService(null)}
                className="text-neutral-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-4 flex-1 overflow-y-auto bg-neutral-950 font-mono text-xs text-neutral-300 whitespace-pre leading-relaxed select-text">
              {logLoading ? (
                <div className="text-center py-8 text-neutral-500">Consultando journalctl...</div>
              ) : (
                logContent
              )}
            </div>

            <div className="px-4 py-3 border-t border-neutral-800 flex justify-between items-center text-xs text-neutral-400 bg-neutral-900">
              <span>Mostrando até 100 linhas mais recentes</span>
              <button
                onClick={() => setSelectedLogService(null)}
                className="px-3 py-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-white font-medium"
              >
                Fechar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
