import { useState } from 'react'
import {
  Server,
  Plus,
  RotateCw,
  Copy,
  Check,
  Trash2,
  ShieldAlert,
  AlertCircle,
  CheckCircle2,
  X,
  Layers,
  Clock,
  Terminal,
} from 'lucide-react'
import { Node, CreateNodeRequest } from '../types/nodes'
import { useServers } from '../context/ServerContext'

interface ServersViewProps {
  onOpenDashboard?: (nodeId: string) => void
  onOpenTerminal?: (nodeId: string) => void
}

export function ServersView({ onOpenDashboard, onOpenTerminal }: ServersViewProps) {
  const { nodes, setActiveNodeId, refreshNodes, isLoading } = useServers()
  const [selectedGroup, setSelectedGroup] = useState<string>('all')
  const [selectedStatus, setSelectedStatus] = useState<string>('all')
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [createdNode, setCreatedNode] = useState<Node | null>(null)
  const [copied, setCopied] = useState(false)
  const [actionMsg, setActionMsg] = useState<{ text: string; isError: boolean } | null>(null)

  const [formData, setFormData] = useState<CreateNodeRequest>({
    name: '',
    group_name: 'Infraestrutura',
    tags: '',
  })

  // Extrai grupos únicos para as abas
  const uniqueGroups = Array.from(new Set(nodes.map((n) => n.group_name).filter(Boolean)))

  const handleCreateNode = async (e: React.FormEvent) => {
    e.preventDefault()
    setActionMsg(null)

    try {
      const res = await fetch('/api/v1/nodes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(formData),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao cadastrar servidor')

      setCreatedNode(data)
      refreshNodes()
      setActionMsg({ text: `Servidor "${data.name}" cadastrado com sucesso!`, isError: false })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Falha na requisição'
      setActionMsg({ text: msg, isError: true })
    }
  }

  const handleRevoke = async (node: Node) => {
    if (
      !window.confirm(
        `Tem certeza de que deseja revogar o acesso do agente "${node.name}"? O servidor será desconectado imediatamente e não poderá mais enviar métricas ou receber comandos até um novo pareamento.`,
      )
    ) {
      return
    }

    try {
      const res = await fetch(`/api/v1/nodes/${node.id}/revoke`, {
        method: 'POST',
        credentials: 'include',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao revogar servidor')

      setActionMsg({ text: `Acesso do servidor "${node.name}" revogado.`, isError: false })
      refreshNodes()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Erro na requisição'
      setActionMsg({ text: msg, isError: true })
    }
  }

  const handleDelete = async (node: Node) => {
    if (
      !window.confirm(
        `Deseja remover "${node.name}" do painel central? Seus arquivos e serviços no host remoto NÃO serão alterados.`,
      )
    ) {
      return
    }

    try {
      const res = await fetch(`/api/v1/nodes/${node.id}`, {
        method: 'DELETE',
        credentials: 'include',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || 'Falha ao excluir servidor')

      setActionMsg({ text: `Servidor "${node.name}" removido com sucesso.`, isError: false })
      refreshNodes()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Erro na requisição'
      setActionMsg({ text: msg, isError: true })
    }
  }

  const copyInstallCmd = (cmd?: string) => {
    if (!cmd) return
    navigator.clipboard.writeText(cmd)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const filteredNodes = nodes.filter((n) => {
    if (selectedGroup !== 'all' && n.group_name !== selectedGroup) return false
    if (selectedStatus !== 'all' && n.status !== selectedStatus) return false
    return true
  })

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
            <Server className="w-6 h-6 text-brand-cyan" />
            Servidores Linux Gerenciados
          </h1>
          <p className="text-sm text-neutral-400">
            Gerenciamento centralizado de nós remotos com pareamento seguro e monitoramento em tempo real.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => refreshNodes()}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-200 text-sm font-medium border border-neutral-700 transition"
          >
            <RotateCw className="w-4 h-4" />
            Atualizar
          </button>
          <button
            onClick={() => {
              setCreatedNode(null)
              setIsModalOpen(true)
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-brand-cyan hover:bg-cyan-500 text-neutral-950 text-sm font-semibold transition shadow-sm"
          >
            <Plus className="w-4 h-4" />
            Adicionar Servidor
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

      {/* Filter Tabs and Status Dropdown */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-neutral-900 border border-neutral-800 p-2.5 rounded-lg">
        {/* Groups Tabs */}
        <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0 scrollbar-none">
          <button
            onClick={() => setSelectedGroup('all')}
            className={`px-3 py-1.5 rounded text-xs font-medium transition shrink-0 ${
              selectedGroup === 'all'
                ? 'bg-brand-cyan text-neutral-950 font-semibold'
                : 'text-neutral-400 hover:text-neutral-200 hover:bg-neutral-800'
            }`}
          >
            Todos os Grupos ({nodes.length})
          </button>
          {uniqueGroups.map((group) => {
            const count = nodes.filter((n) => n.group_name === group).length
            return (
              <button
                key={group}
                onClick={() => setSelectedGroup(group)}
                className={`px-3 py-1.5 rounded text-xs font-medium transition shrink-0 ${
                  selectedGroup === group
                    ? 'bg-brand-cyan text-neutral-950 font-semibold'
                    : 'text-neutral-400 hover:text-neutral-200 hover:bg-neutral-800'
                }`}
              >
                {group} ({count})
              </button>
            )
          })}
        </div>

        {/* State Filter */}
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-xs text-neutral-400">Estado:</span>
          <select
            value={selectedStatus}
            onChange={(e) => setSelectedStatus(e.target.value)}
            className="bg-neutral-950 border border-neutral-800 text-xs text-neutral-200 rounded px-2.5 py-1.5 focus:outline-none focus:border-brand-cyan"
          >
            <option value="all">Todos os Estados</option>
            <option value="online">Online (Conectados)</option>
            <option value="offline">Offline</option>
            <option value="unpaired">Não Pareados</option>
            <option value="revoked">Revogados</option>
          </select>
        </div>
      </div>

      {/* Servers Cards Grid */}
      {isLoading ? (
        <div className="p-12 text-center text-neutral-400 text-sm">Carregando catálogo de servidores...</div>
      ) : filteredNodes.length === 0 ? (
        <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-12 text-center">
          <Layers className="w-10 h-10 text-neutral-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-white">Nenhum servidor encontrado</h3>
          <p className="text-sm text-neutral-400 mt-1 max-w-md mx-auto">
            Cadastre um novo servidor Linux para monitorar recursos e executar operações locais através do Server Agent.
          </p>
          <button
            onClick={() => {
              setCreatedNode(null)
              setIsModalOpen(true)
            }}
            className="mt-4 inline-flex items-center gap-1.5 px-4 py-2 rounded bg-brand-cyan text-neutral-950 text-sm font-medium hover:bg-cyan-500 transition"
          >
            <Plus className="w-4 h-4" />
            Cadastrar Novo Servidor
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {filteredNodes.map((node) => {
            const isOnline = node.status === 'online'
            const isOffline = node.status === 'offline'
            const isUnpaired = node.status === 'unpaired'
            const isRevoked = node.status === 'revoked'

            return (
              <div
                key={node.id}
                className="bg-neutral-900 border border-neutral-800 rounded-lg p-5 flex flex-col justify-between hover:border-neutral-700 transition"
              >
                <div>
                  <div className="flex items-center justify-between">
                    <span
                      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium ${
                        isOnline
                          ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                          : isOffline
                          ? 'bg-rose-500/10 text-rose-400 border border-rose-500/30'
                          : isUnpaired
                          ? 'bg-neutral-800 text-neutral-300 border border-neutral-700'
                          : 'bg-neutral-800 text-neutral-500 border border-neutral-700'
                      }`}
                    >
                      <span
                        className={`w-1.5 h-1.5 rounded-full ${
                          isOnline
                            ? 'bg-emerald-400'
                            : isOffline
                            ? 'bg-rose-500'
                            : isUnpaired
                            ? 'bg-amber-400'
                            : 'bg-neutral-600'
                        }`}
                      />
                      {isOnline
                        ? 'Online'
                        : isOffline
                        ? 'Offline'
                        : isUnpaired
                        ? 'Não Pareado'
                        : isRevoked
                        ? 'Revogado'
                        : node.status}
                    </span>

                    <span className="text-[11px] font-mono px-2 py-0.5 rounded bg-neutral-950 text-neutral-400 border border-neutral-800">
                      {node.group_name}
                    </span>
                  </div>

                  <h3 className="text-base font-bold text-white mt-2.5 truncate" title={node.name}>
                    {node.name}
                  </h3>

                  <div className="text-xs text-neutral-400 mt-0.5 flex items-center gap-2 truncate">
                    <span>{node.hostname || 'Host não informado'}</span>
                    {node.ip_address && <span className="font-mono text-neutral-500">({node.ip_address})</span>}
                  </div>

                  {/* Telemetry Preview */}
                  {node.metrics ? (
                    <div className="mt-4 pt-3 border-t border-neutral-800 space-y-2">
                      <div>
                        <div className="flex justify-between text-[11px] mb-1">
                          <span className="text-neutral-400">CPU:</span>
                          <span className="font-mono text-neutral-200">
                            {node.metrics.cpu_percent.toFixed(1)}%
                          </span>
                        </div>
                        <div className="w-full bg-neutral-950 rounded-full h-1.5 overflow-hidden">
                          <div
                            className={`h-full ${
                              node.metrics.cpu_percent > 80 ? 'bg-rose-500' : 'bg-brand-cyan'
                            }`}
                            style={{ width: `${Math.min(node.metrics.cpu_percent, 100)}%` }}
                          />
                        </div>
                      </div>

                      <div>
                        <div className="flex justify-between text-[11px] mb-1">
                          <span className="text-neutral-400">RAM:</span>
                          <span className="font-mono text-neutral-200">
                            {node.metrics.memory_percent.toFixed(1)}%
                          </span>
                        </div>
                        <div className="w-full bg-neutral-950 rounded-full h-1.5 overflow-hidden">
                          <div
                            className="bg-indigo-500 h-full"
                            style={{ width: `${Math.min(node.metrics.memory_percent, 100)}%` }}
                          />
                        </div>
                      </div>

                      <div className="flex items-center justify-between text-[11px] text-neutral-400 pt-1">
                        <span className="flex items-center gap-1">
                          <Clock className="w-3 h-3 text-neutral-500" />
                          Uptime: {Math.floor(node.metrics.uptime_seconds / 3600)}h
                        </span>
                        <span>Jogos: <strong className="text-neutral-200">{node.metrics.game_servers_count}</strong></span>
                      </div>

                      {node.metrics.is_stale && (
                        <div className="p-1.5 rounded bg-amber-500/10 border border-amber-500/20 text-[10px] text-amber-300 flex items-center gap-1.5">
                          <AlertCircle className="w-3 h-3 shrink-0" />
                          <span>Dados desatualizados ({new Date(node.metrics.collected_at).toLocaleTimeString()})</span>
                        </div>
                      )}
                    </div>
                  ) : (
                    <div className="mt-4 pt-3 border-t border-neutral-800 text-xs text-neutral-500 italic">
                      {isUnpaired ? 'Aguardando pareamento do agente Linux...' : 'Nenhuma métrica coletada ainda.'}
                    </div>
                  )}
                </div>

                <div className="mt-5 pt-3 border-t border-neutral-800 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5">
                    {onOpenDashboard && (
                      <button
                        onClick={() => {
                          setActiveNodeId(node.id)
                          onOpenDashboard(node.id)
                        }}
                        className="px-2.5 py-1 rounded bg-brand-cyan/20 hover:bg-brand-cyan/30 text-brand-cyan text-xs font-semibold transition"
                      >
                        Abrir Dashboard
                      </button>
                    )}

                    {onOpenTerminal && isOnline && (
                      <button
                        onClick={() => {
                          setActiveNodeId(node.id)
                          onOpenTerminal(node.id)
                        }}
                        title="Abrir Terminal no Servidor"
                        className="p-1 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs transition"
                      >
                        <Terminal className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>

                  <div className="flex items-center gap-1">
                    {!isRevoked && (
                      <button
                        onClick={() => handleRevoke(node)}
                        title="Revogar Credencial do Agente"
                        className="p-1 rounded hover:bg-amber-500/20 text-neutral-500 hover:text-amber-400 text-xs transition"
                      >
                        <ShieldAlert className="w-3.5 h-3.5" />
                      </button>
                    )}
                    <button
                      onClick={() => handleDelete(node)}
                      title="Remover Servidor do Catálogo"
                      className="p-1 rounded hover:bg-rose-500/20 text-neutral-500 hover:text-rose-400 text-xs transition"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Modal Adicionar Servidor & Pareamento */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
          <div className="bg-neutral-900 border border-neutral-800 rounded-lg max-w-xl w-full flex flex-col max-h-[90vh] shadow-2xl">
            <div className="flex items-center justify-between px-5 py-4 border-b border-neutral-800">
              <div className="flex items-center gap-2">
                <Server className="w-5 h-5 text-brand-cyan" />
                <h3 className="text-base font-semibold text-white">Cadastrar Novo Servidor Linux</h3>
              </div>
              <button
                onClick={() => {
                  setIsModalOpen(false)
                  setCreatedNode(null)
                }}
                className="text-neutral-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-5 space-y-4 overflow-y-auto">
              {!createdNode ? (
                <form onSubmit={handleCreateNode} className="space-y-4">
                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1">
                      Nome Amigável do Servidor *
                    </label>
                    <input
                      type="text"
                      required
                      placeholder="Ex: Ubuntu-Games-Prod"
                      value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 focus:outline-none focus:border-brand-cyan"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1">
                      Grupo do Servidor
                    </label>
                    <input
                      type="text"
                      placeholder="Ex: Infraestrutura, Jogos Steam, Minecraft, Testes"
                      value={formData.group_name}
                      onChange={(e) => setFormData({ ...formData, group_name: e.target.value })}
                      className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 focus:outline-none focus:border-brand-cyan"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1">
                      Tags / Identificadores (Opcional)
                    </label>
                    <input
                      type="text"
                      placeholder="Ex: ubuntu24, cs2, ssd"
                      value={formData.tags}
                      onChange={(e) => setFormData({ ...formData, tags: e.target.value })}
                      className="w-full bg-neutral-950 border border-neutral-800 text-sm text-neutral-200 rounded px-3 py-2 focus:outline-none focus:border-brand-cyan"
                    />
                  </div>

                  <div className="flex justify-end gap-2 pt-3 border-t border-neutral-800">
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
                      Gerar Pareamento
                    </button>
                  </div>
                </form>
              ) : (
                <div className="space-y-4">
                  <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-lg flex items-start gap-2.5">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />
                    <div>
                      <h4 className="text-sm font-semibold text-emerald-300">
                        Servidor cadastrado! Execute o comando abaixo no host remoto:
                      </h4>
                      <p className="text-xs text-neutral-300 mt-1">
                        O comando instalará o <code>lsm-agent</code>, gerará a identidade Ed25519 local e efetuará o pareamento seguro com este painel central.
                      </p>
                    </div>
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-neutral-400 uppercase tracking-wider mb-1.5">
                      Comando de Instalação e Pareamento (Token expira em 15 minutos)
                    </label>
                    <div className="bg-neutral-950 border border-neutral-800 rounded-lg p-3 flex items-center justify-between gap-3">
                      <code className="font-mono text-xs text-brand-cyan break-all select-all">
                        {createdNode.install_command}
                      </code>
                      <button
                        onClick={() => copyInstallCmd(createdNode.install_command)}
                        className="p-2 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 shrink-0 transition"
                        title="Copiar comando"
                      >
                        {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                      </button>
                    </div>
                  </div>

                  <div className="text-xs text-neutral-500 flex items-center gap-2 pt-2">
                    <span className="w-2 h-2 rounded-full bg-amber-400 animate-pulse" />
                    <span>Aguardando a primeira conexão do agente remoto...</span>
                  </div>

                  <div className="flex justify-end pt-3 border-t border-neutral-800">
                    <button
                      onClick={() => {
                        setIsModalOpen(false)
                        setCreatedNode(null)
                      }}
                      className="px-4 py-2 rounded bg-neutral-800 hover:bg-neutral-700 text-white text-xs font-medium"
                    >
                      Concluir
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
