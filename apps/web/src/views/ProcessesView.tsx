import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Cpu,
  Search,
  RefreshCw,
  Trash2,
  AlertCircle,
  CheckCircle2,
  ShieldAlert,
} from 'lucide-react'
import { ProcessInfo } from '../types/system'
import { useAuth } from '../context/AuthContext'

async function fetchProcesses(): Promise<ProcessInfo[]> {
  const res = await fetch('/api/v1/system/processes', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao listar processos')
  return res.json()
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export function ProcessesView() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [searchTerm, setSearchTerm] = useState('')
  const [sortBy, setSortBy] = useState<'cpu' | 'mem' | 'pid'>('cpu')
  const [actionMsg, setActionMsg] = useState<{ text: string; isError: boolean } | null>(null)

  const { data: processes = [], isLoading, error } = useQuery({
    queryKey: ['system-processes'],
    queryFn: fetchProcesses,
    refetchInterval: 5000,
  })

  const refreshProcs = () => {
    queryClient.invalidateQueries({ queryKey: ['system-processes'] })
  }

  const handleKill = async (proc: ProcessInfo) => {
    if (
      !window.confirm(
        `Tem certeza que deseja encerrar o processo "${proc.name}" (PID: ${proc.pid}) pertencente a "${proc.user}"?`,
      )
    ) {
      return
    }

    try {
      const res = await fetch('/api/v1/system/processes/kill', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ pid: proc.pid, signal: 'SIGTERM' }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao encerrar processo')
      }
      setActionMsg({ text: `Processo PID ${proc.pid} finalizado com sucesso.`, isError: false })
      setTimeout(() => setActionMsg(null), 3000)
      refreshProcs()
    } catch (err: unknown) {
      setActionMsg({
        text: err instanceof Error ? err.message : 'Erro ao encerrar processo',
        isError: true,
      })
      setTimeout(() => setActionMsg(null), 4000)
    }
  }

  const filteredProcs = processes
    .filter(
      (p) =>
        p.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        p.user.toLowerCase().includes(searchTerm.toLowerCase()) ||
        p.pid.toString().includes(searchTerm),
    )
    .sort((a, b) => {
      if (sortBy === 'cpu') return b.cpu_percent - a.cpu_percent
      if (sortBy === 'mem') return b.memory_bytes - a.memory_bytes
      return a.pid - b.pid
    })

  return (
    <div className="space-y-4">
      {/* Top Banner & Control Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <div className="p-2 bg-indigo-500/10 border border-indigo-500/20 rounded-lg text-indigo-400">
            <Cpu className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">Processos em Execução</h2>
            <p className="text-xs text-slate-400">
              Total: {processes.length} processos ativos · Filtrados: {filteredProcs.length}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as 'cpu' | 'mem' | 'pid')}
            className="bg-slate-950 border border-slate-800 text-slate-200 text-xs rounded-lg px-2.5 py-1.5 focus:outline-none focus:border-indigo-500"
          >
            <option value="cpu">Ordenar por CPU</option>
            <option value="mem">Ordenar por Memória</option>
            <option value="pid">Ordenar por PID</option>
          </select>
          <button
            onClick={refreshProcs}
            className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition-colors"
            title="Atualizar lista"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Least Privilege Security Notice */}
      <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-3 flex items-start gap-2.5 text-xs text-slate-400">
        <ShieldAlert className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
        <div>
          <span className="font-semibold text-slate-200">Política de Menor Privilégio Ativa: </span>
          O término de processos opera sob permissões Linux estritas e pidfd. Apenas processos sob a identidade do agente gerenciador (<code className="text-slate-300">lsm-agent</code>) ou processos filhos autorizados podem ser finalizados. Processos de outros operadores do sistema e processos protegidos (<code className="text-slate-300">root</code> e PID 1) não podem ser encerrados pelo painel.
        </div>
      </div>

      {actionMsg && (
        <div
          className={`p-3 rounded-lg text-xs flex items-center gap-2 border ${
            actionMsg.isError
              ? 'bg-rose-500/10 text-rose-300 border-rose-500/20'
              : 'bg-emerald-500/10 text-emerald-300 border-emerald-500/20'
          }`}
        >
          {actionMsg.isError ? (
            <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
          ) : (
            <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
          )}
          <span>{actionMsg.text}</span>
        </div>
      )}

      {/* Filter Input */}
      <div className="relative">
        <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
        <input
          type="text"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          placeholder="Pesquisar por nome do processo, usuário ou PID..."
          className="w-full pl-9 pr-4 py-2 bg-slate-900 border border-slate-800 rounded-xl text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500"
        />
      </div>

      {/* Processes Table */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
        {isLoading ? (
          <div className="p-8 text-center text-xs text-slate-400">Carregando processos...</div>
        ) : error ? (
          <div className="p-8 text-center text-xs text-rose-400">Falha ao obter processos</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 text-slate-400 bg-slate-950/40">
                  <th className="py-2.5 px-4 font-medium">PID</th>
                  <th className="py-2.5 px-4 font-medium">Nome</th>
                  <th className="py-2.5 px-4 font-medium">Usuário</th>
                  <th className="py-2.5 px-4 font-medium">CPU (%)</th>
                  <th className="py-2.5 px-4 font-medium">Memória</th>
                  <th className="py-2.5 px-4 font-medium">Estado</th>
                  <th className="py-2.5 px-4 font-medium text-right">Ação</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {filteredProcs.map((proc) => {
                  const canKill = user?.is_admin || proc.uid === user?.uid
                  return (
                    <tr key={proc.pid} className="hover:bg-slate-800/40 transition-colors">
                      <td className="py-2.5 px-4 font-mono text-slate-400">{proc.pid}</td>
                      <td className="py-2.5 px-4">
                        <span className="font-semibold text-white block">{proc.name}</span>
                        <span className="font-mono text-[10px] text-slate-500 truncate block max-w-xs">
                          {proc.command}
                        </span>
                      </td>
                      <td className="py-2.5 px-4">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-mono ${
                            proc.user === 'root'
                              ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                              : 'bg-slate-800 text-slate-300'
                          }`}
                        >
                          {proc.user}
                        </span>
                      </td>
                      <td className="py-2.5 px-4 font-mono font-medium">
                        <span
                          className={
                            proc.cpu_percent > 10
                              ? 'text-rose-400'
                              : proc.cpu_percent > 3
                              ? 'text-amber-400'
                              : 'text-slate-300'
                          }
                        >
                          {proc.cpu_percent.toFixed(1)}%
                        </span>
                      </td>
                      <td className="py-2.5 px-4 font-mono text-slate-400">
                        {formatBytes(proc.memory_bytes)}
                      </td>
                      <td className="py-2.5 px-4">
                        <span className="text-[11px] text-slate-400">{proc.state}</span>
                      </td>
                      <td className="py-2.5 px-4 text-right">
                        <button
                          onClick={() => handleKill(proc)}
                          disabled={!canKill}
                          className="p-1.5 rounded hover:bg-rose-950/60 text-slate-400 hover:text-rose-400 disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-slate-400 transition-colors"
                          title={
                            canKill
                              ? 'Finalizar processo'
                              : 'Sem permissão para encerrar processos de outros usuários'
                          }
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
