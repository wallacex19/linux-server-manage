import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  Cpu,
  HardDrive,
  RefreshCw,
  Server,
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

async function fetchMetricsHistory(): Promise<ServerMetricsSnapshot[]> {
  const res = await fetch('/api/v1/metrics/history', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao obter histórico de métricas')
  return res.json()
}

async function fetchCurrentMetrics(): Promise<ServerMetricsSnapshot> {
  const res = await fetch('/api/v1/metrics/current', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao obter métricas atuais')
  return res.json()
}

export function MonitoringView() {
  const [intervalMs, setIntervalMs] = useState<number | false>(3000)

  const { data: current } = useQuery({
    queryKey: ['metrics-current'],
    queryFn: fetchCurrentMetrics,
    refetchInterval: intervalMs,
  })

  const { data: history = [] } = useQuery({
    queryKey: ['metrics-history'],
    queryFn: fetchMetricsHistory,
    refetchInterval: intervalMs,
  })

  const chartData = history.map((snap) => {
    const timeLabel = new Date(snap.timestamp).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
    const netRx = snap.network.reduce((acc, i) => acc + i.rx_bytes_per_sec, 0) / 1024
    const netTx = snap.network.reduce((acc, i) => acc + i.tx_bytes_per_sec, 0) / 1024
    const diskRead = snap.disk.read_bytes_per_sec / 1024
    const diskWrite = snap.disk.write_bytes_per_sec / 1024

    return {
      time: timeLabel,
      cpu: snap.cpu.total_percent,
      memory: snap.memory.used_percent,
      swap: snap.memory.swap_used_percent,
      netRx: Math.round(netRx * 10) / 10,
      netTx: Math.round(netTx * 10) / 10,
      diskRead: Math.round(diskRead * 10) / 10,
      diskWrite: Math.round(diskWrite * 10) / 10,
    }
  })

  const perCoreData =
    current?.cpu.per_core_percent?.map((usage, idx) => ({
      core: `Core ${idx}`,
      usage: usage,
    })) || []

  return (
    <div className="space-y-6">
      {/* Top Banner */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div>
          <h2 className="text-base font-semibold text-white">Monitoramento de Recursos em Tempo Real</h2>
          <p className="text-xs text-slate-400 mt-1">
            Métricas de baixo overhead coletadas de interfaces nativas do Linux (/proc, /sys)
          </p>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 text-xs text-slate-400">
            <RefreshCw
              className={`w-3.5 h-3.5 text-indigo-400 ${intervalMs ? 'animate-spin' : ''}`}
              style={{ animationDuration: '4s' }}
            />
            <span>Taxa de amostragem:</span>
            <select
              value={intervalMs === false ? 'pause' : intervalMs}
              onChange={(e) => {
                const val = e.target.value
                setIntervalMs(val === 'pause' ? false : parseInt(val))
              }}
              className="bg-slate-950 border border-slate-800 text-slate-200 text-xs rounded-md px-2 py-1 focus:outline-none focus:border-indigo-500"
            >
              <option value="2000">2s (Alta resolução)</option>
              <option value="3000">3s (Padrão)</option>
              <option value="5000">5s (Econômico)</option>
              <option value="10000">10s</option>
              <option value="pause">Pausar</option>
            </select>
          </div>
        </div>
      </div>

      {/* Grid of Dedicated Real-Time Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Full CPU History */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Cpu className="w-4 h-4 text-indigo-400" />
              <h3 className="text-sm font-semibold text-white">Carga de CPU (%)</h3>
            </div>
            <span className="text-xs font-mono text-indigo-400">
              Atual: {current?.cpu.total_percent.toFixed(1)}%
            </span>
          </div>
          <div className="h-60 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="cpuArea" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#6366f1" stopOpacity={0.5} />
                    <stop offset="95%" stopColor="#6366f1" stopOpacity={0.0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                <XAxis dataKey="time" stroke="#64748b" tick={{ fontSize: 10 }} />
                <YAxis domain={[0, 100]} stroke="#64748b" tick={{ fontSize: 10 }} unit="%" />
                <Tooltip contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }} />
                <Area type="monotone" dataKey="cpu" name="CPU" stroke="#6366f1" strokeWidth={2} fill="url(#cpuArea)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Per-Core Distribution */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Server className="w-4 h-4 text-emerald-400" />
              <h3 className="text-sm font-semibold text-white">Uso por Núcleo Individual</h3>
            </div>
            <span className="text-xs text-slate-400">
              {current?.cpu.cores} núcleos ativos
            </span>
          </div>
          <div className="h-60 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={perCoreData} margin={{ top: 10, right: 10, left: -25, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                <XAxis dataKey="core" stroke="#64748b" tick={{ fontSize: 10 }} />
                <YAxis domain={[0, 100]} stroke="#64748b" tick={{ fontSize: 10 }} unit="%" />
                <Tooltip contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }} />
                <Bar dataKey="usage" name="Uso" fill="#10b981" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* RAM & Swap Chart */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Activity className="w-4 h-4 text-emerald-400" />
              <h3 className="text-sm font-semibold text-white">Consumo de Memória (RAM & Swap)</h3>
            </div>
            <div className="flex items-center gap-3 text-xs text-slate-400">
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 inline-block" />
                RAM ({current?.memory.used_percent.toFixed(0)}%)
              </span>
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-amber-500 inline-block" />
                Swap ({current?.memory.swap_used_percent.toFixed(0)}%)
              </span>
            </div>
          </div>
          <div className="h-60 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                <XAxis dataKey="time" stroke="#64748b" tick={{ fontSize: 10 }} />
                <YAxis domain={[0, 100]} stroke="#64748b" tick={{ fontSize: 10 }} unit="%" />
                <Tooltip contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }} />
                <Line type="monotone" dataKey="memory" name="RAM" stroke="#10b981" strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="swap" name="Swap" stroke="#f59e0b" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Disk I/O Activity Chart */}
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <HardDrive className="w-4 h-4 text-amber-400" />
              <h3 className="text-sm font-semibold text-white">Throughput de Disco (KB/s)</h3>
            </div>
            <div className="flex items-center gap-3 text-xs text-slate-400">
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-blue-400 inline-block" />
                Leitura
              </span>
              <span className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full bg-amber-400 inline-block" />
                Gravação
              </span>
            </div>
          </div>
          <div className="h-60 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ top: 10, right: 10, left: -10, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                <XAxis dataKey="time" stroke="#64748b" tick={{ fontSize: 10 }} />
                <YAxis stroke="#64748b" tick={{ fontSize: 10 }} unit=" KB" />
                <Tooltip contentStyle={{ backgroundColor: '#0f172a', borderColor: '#1e293b', fontSize: '12px' }} />
                <Line type="monotone" dataKey="diskRead" name="Leitura" stroke="#60a5fa" strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="diskWrite" name="Gravação" stroke="#f59e0b" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  )
}
