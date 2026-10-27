import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Gamepad2,
  CheckCircle2,
  AlertTriangle,
  RotateCw,
  Download,
  Copy,
  Check,
  ShieldCheck,
} from 'lucide-react'
import { SteamCMDStatus, PopularGamePreset } from '../types/steamcmd'

async function fetchStatus(): Promise<SteamCMDStatus> {
  const res = await fetch('/api/v1/steamcmd/status', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao verificar status do SteamCMD')
  return res.json()
}

async function fetchPresets(): Promise<PopularGamePreset[]> {
  const res = await fetch('/api/v1/steamcmd/presets', { credentials: 'include' })
  if (!res.ok) throw new Error('Falha ao obter presets de jogos')
  return res.json()
}

interface SteamCMDViewProps {
  onNavigateToCreate?: (preset?: PopularGamePreset) => void
}

export function SteamCMDView({ onNavigateToCreate }: SteamCMDViewProps) {
  const [copied, setCopied] = useState(false)

  const {
    data: status,
    isLoading: statusLoading,
    refetch: refetchStatus,
  } = useQuery({
    queryKey: ['steamcmd-status'],
    queryFn: fetchStatus,
  })

  const { data: presets = [], isLoading: presetsLoading } = useQuery({
    queryKey: ['steamcmd-presets'],
    queryFn: fetchPresets,
  })

  const copyUbuntuCmd = () => {
    const cmd = 'sudo dpkg --add-architecture i386 && sudo apt-get update && sudo apt-get install -y steamcmd lib32gcc-s1'
    navigator.clipboard.writeText(cmd)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
            <Gamepad2 className="w-6 h-6 text-brand-cyan" />
            SteamCMD e Servidores de Jogos
          </h1>
          <p className="text-sm text-neutral-400">
            Gerenciamento e automação de ferramentas Valve SteamCMD e instâncias dedicadas.
          </p>
        </div>

        <button
          onClick={() => refetchStatus()}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-200 text-sm font-medium border border-neutral-700 transition"
        >
          <RotateCw className="w-4 h-4" />
          Verificar Status
        </button>
      </div>

      {/* SteamCMD Status Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* Status Card */}
        <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-neutral-400 uppercase tracking-wider">
              Binário SteamCMD
            </span>
            {status?.installed ? (
              <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                <CheckCircle2 className="w-3.5 h-3.5" />
                Instalado
              </span>
            ) : (
              <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/30">
                <AlertTriangle className="w-3.5 h-3.5" />
                Não Detectado
              </span>
            )}
          </div>
          <div className="mt-3">
            <div className="text-sm font-mono text-white truncate" title={status?.path || 'Não encontrado'}>
              {statusLoading ? 'Verificando...' : status?.path || 'Não encontrado no $PATH'}
            </div>
            <div className="text-xs text-neutral-500 mt-1 truncate" title={status?.version}>
              {status?.version || 'Nenhuma versão detectada'}
            </div>
          </div>
        </div>

        {/* 32-bit Compatibility Card */}
        <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-neutral-400 uppercase tracking-wider">
              Bibliotecas 32-Bit (Ubuntu)
            </span>
            {status?.libs_32bit_ready ? (
              <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                <ShieldCheck className="w-3.5 h-3.5" />
                Pronto (lib32gcc-s1)
              </span>
            ) : (
              <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/30">
                <AlertTriangle className="w-3.5 h-3.5" />
                Pendente
              </span>
            )}
          </div>
          <div className="mt-3">
            <div className="text-sm text-neutral-200">
              {status?.libs_32bit_ready
                ? 'Ambiente pronto para executar binários 32-bit e 64-bit'
                : 'Necessário instalar lib32gcc-s1 para execução de jogos Valve'}
            </div>
            <div className="text-xs text-neutral-500 mt-1">
              SO: {status?.os_distribution || 'Linux'}
            </div>
          </div>
        </div>

        {/* Security Isolation Card */}
        <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-neutral-400 uppercase tracking-wider">
              Isolamento e Segurança
            </span>
            <span className="text-xs px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/30">
              Least Privilege
            </span>
          </div>
          <div className="mt-3">
            <div className="text-sm text-neutral-200">
              Servidores rodam estritamente sob sessões do usuário autenticado.
            </div>
            <div className="text-xs text-neutral-500 mt-1">
              Zero execução global como superusuário (root).
            </div>
          </div>
        </div>
      </div>

      {/* Ubuntu 24.04 LTS Installation Helper */}
      {(!status?.installed || !status?.libs_32bit_ready) && (
        <div className="bg-neutral-900/90 border border-amber-500/30 rounded-lg p-5">
          <h3 className="text-sm font-semibold text-amber-300 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400" />
            Guia de Instalação no Host Ubuntu Server 24.04 LTS
          </h3>
          <p className="text-xs text-neutral-300 mt-1.5 leading-relaxed">
            O SteamCMD necessita da arquitetura 32-bit (i386) e da biblioteca de suporte{' '}
            <code className="text-amber-200 bg-neutral-950 px-1 py-0.5 rounded">lib32gcc-s1</code>. Execute o
            comando abaixo no terminal do servidor com privilégios administrativos:
          </p>

          <div className="mt-3 flex items-center justify-between bg-neutral-950 border border-neutral-800 rounded-md p-3">
            <code className="font-mono text-xs text-brand-cyan select-all">
              sudo dpkg --add-architecture i386 && sudo apt-get update && sudo apt-get install -y steamcmd lib32gcc-s1
            </code>
            <button
              onClick={copyUbuntuCmd}
              className="ml-3 p-1.5 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 transition shrink-0"
              title="Copiar comando"
            >
              {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
        </div>
      )}

      {/* Popular Game Server Presets */}
      <div className="bg-neutral-900 border border-neutral-800 rounded-lg p-5">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-base font-semibold text-white">Catálogo de Jogos Suportados</h3>
            <p className="text-xs text-neutral-400">
              Modelos pré-configurados com AppID oficial, portas padrão e argumentos recomendados.
            </p>
          </div>
        </div>

        {presetsLoading ? (
          <div className="p-8 text-center text-neutral-400 text-sm">Carregando catálogo...</div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {presets.map((preset) => (
              <div
                key={preset.app_id}
                className="bg-neutral-950 border border-neutral-800 rounded-lg p-4 flex flex-col justify-between hover:border-neutral-700 transition"
              >
                <div>
                  <div className="flex items-center justify-between">
                    <h4 className="text-sm font-semibold text-white">{preset.name}</h4>
                    <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-neutral-800 text-neutral-300">
                      AppID: {preset.app_id}
                    </span>
                  </div>
                  <p className="text-xs text-neutral-400 mt-1">{preset.description}</p>

                  <div className="mt-3 space-y-1 text-[11px] text-neutral-400">
                    <div>
                      <span className="text-neutral-500">Porta padrão:</span>{' '}
                      <span className="font-mono text-neutral-300">{preset.default_port}/UDP</span>
                    </div>
                    <div>
                      <span className="text-neutral-500">Login Steam:</span>{' '}
                      <span className="font-mono text-neutral-300">
                        {preset.anonymous ? 'Anônimo (sem conta Valve)' : 'Requer login'}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="mt-4 pt-3 border-t border-neutral-800 flex items-center justify-between">
                  <span className="text-[10px] text-neutral-500 font-mono">{preset.service_type}</span>
                  {onNavigateToCreate && (
                    <button
                      onClick={() => onNavigateToCreate(preset)}
                      className="flex items-center gap-1.5 px-2.5 py-1 rounded bg-brand-cyan/20 hover:bg-brand-cyan/30 text-brand-cyan text-xs font-medium transition"
                    >
                      <Download className="w-3.5 h-3.5" />
                      Criar Servidor
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
