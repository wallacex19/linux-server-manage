import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '../context/AuthContext'
import { useServers } from '../context/ServerContext'
import {
  Activity,
  HardDrive,
  Clock,
  Shield,
  Layers,
  Terminal,
  Folder,
  DownloadCloud,
  Cpu,
  Network,
  Thermometer,
  ArrowDown,
  ArrowUp,
  RefreshCw,
  Server,
  Globe,
} from 'lucide-react'
import {
  AreaChart,
  Area,
  LineChart,
  Line,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts'
import { ServerMetricsSnapshot } from '../types/metrics'

async function fetchCurrentMetrics(): Promise<ServerMetricsSnapshot> {
  const res = await fetch('/api/v1/metrics/current', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao obter métricas atuais')
  return res.json()
}

async function fetchMetricsHistory(): Promise<ServerMetricsSnapshot[]> {
  const res = await fetch('/api/v1/metrics/history', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao obter histórico de métricas')
  return res.json()
}

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}

function formatRate(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec === 0) return '0 KB/s'
  const kb = bytesPerSec / 1024
  if (kb > 1024) {
    return `${(kb / 1024).toFixed(1)} MB/s`
  }
  return `${kb.toFixed(1)} KB/s`
}

function formatUptime(seconds?: number): string {
  if (!seconds) return '—'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h ${mins}m`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m ${seconds % 60}s`
}

interface DashboardViewProps {
  onNavigate: (viewId: string) => void
}

export function DashboardView({ onNavigate }: DashboardViewProps) {
  const { user } = useAuth()
  const { nodes, activeNode } = useServers()
  const [intervalMs, setIntervalMs] = useState<number | false>(3000)

  const safeNodes = Array.isArray(nodes) ? nodes : []
  const onlineCount = safeNodes.filter((n) => n.status === 'online').length
  const totalGames = safeNodes.reduce((acc, n) => acc + (n.metrics?.game_servers_count || 0), 0)

  const { data: current, isLoading: currentLoading } = useQuery({
    queryKey: ['metrics-current'],
    queryFn: fetchCurrentMetrics,
    refetchInterval: intervalMs,
  })

  const { data: history = [] } = useQuery({
    queryKey: ['metrics-history'],
    queryFn: fetchMetricsHistory,
    refetchInterval: intervalMs,
  })

  // Prepara dados para os gráficos históricos
  const chartData = history.map((snap) => {
    const timeLabel = new Date(snap.timestamp).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
    const netRx = snap.network.reduce((acc, iface) => acc + iface.rx_bytes_per_sec, 0) / 1024
    const netTx = snap.network.reduce((acc, iface) => acc + iface.tx_bytes_per_sec, 0) / 1024

    return {
      time: timeLabel,
      cpu: snap.cpu.total_percent,
      memory: snap.memory.used_percent,
      netRx: Math.round(netRx * 10) / 10,
      netTx: Math.round(netTx * 10) / 10,
    }
  })

  // Dados para o gráfico de CPU por núcleo
  const perCoreData =
    current?.cpu.per_core_percent?.map((usage, idx) => ({
      core: `Core ${idx}`,
      usage: usage,
    })) || []

  // Agrega tráfego de rede atual
  const totalRxRate = current?.network?.reduce((acc, i) => acc + i.rx_bytes_per_sec, 0) || 0
  const totalTxRate = current?.network?.reduce((acc, i) => acc + i.tx_bytes_per_sec, 0) || 0

  return (
    <div className="space-y-6">
      {/* Top Banner & Control Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            {activeNode ? (
              <>
                <Server className="w-5 h-5 text-brand-cyan" />
                <h2 className="text-base font-semibold text-white">
                  {activeNode.name}
                </h2>
                <span className="px-2 py-0.5 rounded text-[11px] bg-slate-800 text-slate-300 border border-slate-700">
                  {activeNode.hostname || activeNode.ip_address || 'Host Remoto'}
                </span>
                <span className="px-2 py-0.5 rounded text-[10px] bg-brand-cyan/20 text-brand-cyan border border-brand-cyan/30">
                  {activeNode.group_name}
                </span>
              </>
            ) : (
              <>
                <Globe className="w-5 h-5 text-brand-cyan" />
                <h2 className="text-base font-semibold text-white">
                  Dashboard Consolidado (Visão Global)
                </h2>
                <span className="px-2 py-0.5 rounded text-[11px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-medium">
                  {onlineCount} de {safeNodes.length} Servidores Online
                </span>
              </>
            )}
          </div>
          <p className="text-xs text-slate-400 mt-1">
            {activeNode
              ? `Sistema: ${activeNode.os_info || 'Linux'} · Agente v${activeNode.agent_version || '1.0'}`
              : `${safeNodes.length} nós cadastrados · ${totalGames} servidores de jogos dedicados gerenciados`}
            {' · '}Autenticado como <strong className="text-slate-200">{user?.username}</strong>
          </p>
        </div>

        {/* Polling Interval Controls */}
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 text-xs text-slate-400">
            <RefreshCw
              className={`w-3.5 h-3.5 text-indigo-400 ${
                intervalMs ? 'animate-spin' : ''
              }`}
              style={{ animationDuration: '4s' }}
            />
            <span>Atualização:</span>
            <select
              value={intervalMs === false ? 'pause' : intervalMs}
              onChange={(e) => {
                const val = e.target.value
                setIntervalMs(val === 'pause' ? false : parseInt(val))
              }}
              className="bg-slate-950 border border-slate-800 text-slate-200 text-xs rounded-md px-2 py-1 focus:outline-none focus:border-indigo-500"
            >
              <option value="2000">2s (Rápido)</option>
              <option value="3000">3s (Padrão)</option>
              <option value="5000">5s (Econômico)</option>
              <option value="10000">10s</option>
              <option value="pause">Pausar</option>
            </select>
          </div>
        </div>
      </div>

      {/* Main KPI Cards Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* CPU Card */}
        <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400">
            <span className="text-xs font-medium">Uso Total de CPU</span>
            <Cpu className="w-4 h-4 text-indigo-400" />
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-2xl font-bold text-white">
              {currentLoading ? '...' : `${current?.cpu.total_percent.toFixed(1)}%`}
            </span>
            <span className="text-xs text-slate-400">
              {current?.cpu.cores} núcleos
            </span>
          </div>
          <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                (current?.cpu.total_percent || 0) > 85
                  ? 'bg-rose-500'
                  : (current?.cpu.total_percent || 0) > 65
                  ? 'bg-amber-500'
                  : 'bg-indigo-500'
              }`}
              style={{ width: `${Math.min(100, current?.cpu.total_percent || 0)}%` }}
            />
          </div>
          <span className="text-[11px] text-slate-400 block truncate">
            Load: {current?.system.load_1.toFixed(2)} · {current?.system.load_5.toFixed(2)} ·{' '}
            {current?.system.load_15.toFixed(2)}
          </span>
        </div>

        {/* Memory RAM & Swap Card */}
        <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400">
            <span className="text-xs font-medium">Memória RAM</span>
            <Activity className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-2xl font-bold text-white">
              {currentLoading ? '...' : `${current?.memory.used_percent.toFixed(1)}%`}
            </span>
            <span className="text-xs text-slate-400">
              {formatBytes(current?.memory.used_bytes || 0)} / {formatBytes(current?.memory.total_bytes || 0)}
            </span>
          </div>
          <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                (current?.memory.used_percent || 0) > 85 ? 'bg-rose-500' : 'bg-emerald-500'
              }`}
              style={{ width: `${Math.min(100, current?.memory.used_percent || 0)}%` }}
            />
          </div>
          <span className="text-[11px] text-slate-400 block truncate">
            Swap: {formatBytes(current?.memory.swap_used_bytes || 0)} ({current?.memory.swap_used_percent.toFixed(0)}%)
          </span>
        </div>

        {/* Disk I/O & Storage Card */}
        <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400">
            <span className="text-xs font-medium">Atividade de Disco</span>
            <HardDrive className="w-4 h-4 text-amber-400" />
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-xl font-bold text-white">
              {formatRate(current?.disk.write_bytes_per_sec || 0)}
            </span>
            <span className="text-xs text-slate-400">Gravação</span>
          </div>
          <div className="flex items-center justify-between text-xs text-slate-300 pt-0.5">
            <span className="flex items-center gap-1 text-[11px] text-slate-400">
              Leitura: {formatRate(current?.disk.read_bytes_per_sec || 0)}
            </span>
            <span className="text-[11px] text-slate-400">
              {current?.disk.partitions?.[0]?.mount_point || '/'}:{' '}
              {current?.disk.partitions?.[0]?.used_percent.toFixed(0)}%
            </span>
          </div>
          <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
            <div
              className="h-full bg-amber-500 rounded-full transition-all duration-500"
              style={{ width: `${current?.disk.partitions?.[0]?.used_percent || 0}%` }}
            />
          </div>
        </div>

        {/* Network & Temperature Card */}
        <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400">
            <span className="text-xs font-medium">Rede & Hardware</span>
            <Network className="w-4 h-4 text-purple-400" />
          </div>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5 text-xs text-slate-200">
              <ArrowDown className="w-3.5 h-3.5 text-emerald-400" />
              <span>{formatRate(totalRxRate)}</span>
            </div>
            <div className="flex items-center gap-1.5 text-xs text-slate-200">
              <ArrowUp className="w-3.5 h-3.5 text-indigo-400" />
              <span>{formatRate(totalTxRate)}</span>
            </div>
          </div>
          <div className="flex items-center justify-between pt-1 border-t border-slate-800 text-[11px] text-slate-400">
            <span className="flex items-center gap-1">
              <Clock className="w-3 h-3 text-slate-400" />
              {formatUptime(current?.system.uptime_seconds)}
            </span>
            <span className="flex items-center gap-1">
              <Thermometer className="w-3 h-3 text-rose-400" />
              {current?.system.hardware_temperature_celsius !== null &&
              current?.system.hardware_temperature_celsius !== undefined
                ? `${current.system.hardware_temperature_celsius.toFixed(1)}°C`
                : 'N/A (VM)'}
            </span>
          </div>
          <span className="text-[10px] text-slate-500 block truncate">
            {current?.system.process_count || 0} processos no host
          </span>
        </div>
      </div>

      {/* Historical Trend Charts Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* CPU & Memory History Chart */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Activity className="w-4 h-4 text-indigo-400" />
              <h3 className="text-sm font-semibold text-white">
                Histórico de CPU e Memória RAM
              </h3>
            </div>
            <div className="flex items-center gap-3 text-xs text-slate-400">
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-indigo-500 inline-block" />
                CPU (%)
              </span>
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 inline-block" />
                RAM (%)
              </span>
            </div>
          </div>

          <div className="h-56 w-full">
            {chartData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="cpuGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#6366f1" stopOpacity={0.4} />
                      <stop offset="95%" stopColor="#6366f1" stopOpacity={0.0} />
                    </linearGradient>
                    <linearGradient id="memGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#10b981" stopOpacity={0.0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                  <XAxis dataKey="time" stroke="#64748b" tick={{ fontSize: 10 }} />
                  <YAxis domain={[0, 100]} stroke="#64748b" tick={{ fontSize: 10 }} unit="%" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }}
                  />
                  <Area
                    type="monotone"
                    dataKey="cpu"
                    name="CPU"
                    stroke="#6366f1"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#cpuGradient)"
                  />
                  <Area
                    type="monotone"
                    dataKey="memory"
                    name="RAM"
                    stroke="#10b981"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#memGradient)"
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-xs text-slate-500">
                Aguardando primeiras amostras...
              </div>
            )}
          </div>
        </div>

        {/* Network Throughput History Chart */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Network className="w-4 h-4 text-purple-400" />
              <h3 className="text-sm font-semibold text-white">
                Taxa de Transferência de Rede (KB/s)
              </h3>
            </div>
            <div className="flex items-center gap-3 text-xs text-slate-400">
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 inline-block" />
                Download (Rx)
              </span>
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-purple-400 inline-block" />
                Upload (Tx)
              </span>
            </div>
          </div>

          <div className="h-56 w-full">
            {chartData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData} margin={{ top: 10, right: 10, left: -10, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                  <XAxis dataKey="time" stroke="#64748b" tick={{ fontSize: 10 }} />
                  <YAxis stroke="#64748b" tick={{ fontSize: 10 }} unit=" KB" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }}
                  />
                  <Line
                    type="monotone"
                    dataKey="netRx"
                    name="Download"
                    stroke="#34d399"
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    type="monotone"
                    dataKey="netTx"
                    name="Upload"
                    stroke="#a855f7"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-xs text-slate-500">
                Aguardando amostras de rede...
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Per-Core Breakdown & Filesystem Table */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Per-Core CPU Bars */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center gap-2">
            <Cpu className="w-4 h-4 text-indigo-400" />
            <h3 className="text-sm font-semibold text-white">Uso por Núcleo de CPU</h3>
          </div>
          <div className="h-48 w-full">
            {perCoreData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={perCoreData} margin={{ top: 10, right: 10, left: -25, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                  <XAxis dataKey="core" stroke="#64748b" tick={{ fontSize: 10 }} />
                  <YAxis domain={[0, 100]} stroke="#64748b" tick={{ fontSize: 10 }} unit="%" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }}
                  />
                  <Bar dataKey="usage" name="Uso" fill="#6366f1" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-xs text-slate-500">
                Detecção por núcleo não disponível
              </div>
            )}
          </div>
        </div>

        {/* Filesystem Mounts Table */}
        <div className="lg:col-span-2 p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <HardDrive className="w-4 h-4 text-amber-400" />
              <h3 className="text-sm font-semibold text-white">Sistemas de Arquivos & Partições</h3>
            </div>
            <span className="text-xs text-slate-400">
              {current?.disk.partitions?.length || 0} partição(ões)
            </span>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 text-slate-400">
                  <th className="pb-2 font-medium">Ponto de Montagem</th>
                  <th className="pb-2 font-medium">Dispositivo</th>
                  <th className="pb-2 font-medium">Tipo</th>
                  <th className="pb-2 font-medium">Tamanho Total</th>
                  <th className="pb-2 font-medium">Disponível</th>
                  <th className="pb-2 font-medium text-right">Uso (%)</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {current?.disk.partitions && current.disk.partitions.length > 0 ? (
                  current.disk.partitions.map((part) => (
                    <tr key={part.mount_point}>
                      <td className="py-2.5 font-medium text-white">{part.mount_point}</td>
                      <td className="py-2.5 font-mono text-slate-400">{part.device}</td>
                      <td className="py-2.5 text-slate-400">{part.fs_type}</td>
                      <td className="py-2.5">{formatBytes(part.total_bytes)}</td>
                      <td className="py-2.5 text-emerald-400">{formatBytes(part.free_bytes)}</td>
                      <td className="py-2.5 text-right font-medium">
                        <span
                          className={
                            part.used_percent > 85
                              ? 'text-rose-400'
                              : part.used_percent > 70
                              ? 'text-amber-400'
                              : 'text-slate-200'
                          }
                        >
                          {part.used_percent.toFixed(1)}%
                        </span>
                      </td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td colSpan={6} className="py-4 text-center text-slate-500">
                      Nenhuma partição encontrada
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Identity & Module Quick Launch */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* User Identity Details Card */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center gap-2">
            <Shield className="w-4 h-4 text-indigo-400" />
            <h3 className="text-sm font-semibold text-white">Identidade Linux Efetiva</h3>
          </div>
          <p className="text-xs text-slate-400 leading-relaxed">
            Sessão validada via PAM. Todas as consultas e operações respeitam o UID do seu usuário no host Linux.
          </p>
          <div className="space-y-2 text-xs">
            <div className="flex justify-between py-1.5 border-b border-slate-800">
              <span className="text-slate-400">Usuário:</span>
              <span className="font-mono text-slate-200">{user?.username}</span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-800">
              <span className="text-slate-400">UID / GID:</span>
              <span className="font-mono text-slate-200">
                {user?.uid} / {user?.gid}
              </span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-800">
              <span className="text-slate-400">Grupos:</span>
              <span className="font-mono text-slate-200 truncate max-w-[150px]">
                {user?.groups?.join(', ') || 'nenhum'}
              </span>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-400">Privilégios:</span>
              <span className={user?.is_admin ? 'text-indigo-400 font-semibold' : 'text-slate-300'}>
                {user?.is_admin ? 'Administrador (sudo)' : 'Operador Não Privilegiado'}
              </span>
            </div>
          </div>
        </div>

        {/* Quick Launchpad to Project Modules */}
        <div className="lg:col-span-2 p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center gap-2">
            <Activity className="w-4 h-4 text-emerald-400" />
            <h3 className="text-sm font-semibold text-white">Acesso Rápido aos Módulos</h3>
          </div>
          <p className="text-xs text-slate-400">
            Navegue pelos módulos de administração e gerenciamento de servidores:
          </p>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-1">
            <button
              onClick={() => onNavigate('terminal')}
              className="p-3 bg-slate-950/60 hover:bg-slate-800/80 border border-slate-800 rounded-lg text-left transition-colors flex flex-col gap-2 group"
            >
              <Terminal className="w-5 h-5 text-indigo-400 group-hover:scale-105 transition-transform" />
              <div>
                <span className="text-xs font-medium text-white block">Terminal</span>
                <span className="text-[10px] text-slate-500">PTY e xterm.js</span>
              </div>
            </button>

            <button
              onClick={() => onNavigate('filemanager')}
              className="p-3 bg-slate-950/60 hover:bg-slate-800/80 border border-slate-800 rounded-lg text-left transition-colors flex flex-col gap-2 group"
            >
              <Folder className="w-5 h-5 text-emerald-400 group-hover:scale-105 transition-transform" />
              <div>
                <span className="text-xs font-medium text-white block">Arquivos</span>
                <span className="text-[10px] text-slate-500">Editor e uploads</span>
              </div>
            </button>

            <button
              onClick={() => onNavigate('processes')}
              className="p-3 bg-slate-950/60 hover:bg-slate-800/80 border border-slate-800 rounded-lg text-left transition-colors flex flex-col gap-2 group"
            >
              <Layers className="w-5 h-5 text-amber-400 group-hover:scale-105 transition-transform" />
              <div>
                <span className="text-xs font-medium text-white block">Processos</span>
                <span className="text-[10px] text-slate-500">systemd & jobs</span>
              </div>
            </button>

            <button
              onClick={() => onNavigate('steamcmd')}
              className="p-3 bg-slate-950/60 hover:bg-slate-800/80 border border-slate-800 rounded-lg text-left transition-colors flex flex-col gap-2 group"
            >
              <DownloadCloud className="w-5 h-5 text-purple-400 group-hover:scale-105 transition-transform" />
              <div>
                <span className="text-xs font-medium text-white block">SteamCMD</span>
                <span className="text-[10px] text-slate-500">Servidores de jogos</span>
              </div>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
