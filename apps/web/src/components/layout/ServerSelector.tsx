import { useState, useRef, useEffect } from 'react'
import { Server, ChevronDown, Check, Globe } from 'lucide-react'
import { useServers } from '../../context/ServerContext'

interface ServerSelectorProps {
  onNavigateToServers?: () => void
}

export function ServerSelector({ onNavigateToServers }: ServerSelectorProps) {
  const { nodes, activeNodeId, activeNode, setActiveNodeId } = useServers()
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node as unknown as globalThis.Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const getStatusColor = (status?: string) => {
    switch (status) {
      case 'online':
        return 'bg-emerald-400'
      case 'offline':
        return 'bg-rose-500'
      case 'connecting':
        return 'bg-amber-400 animate-pulse'
      case 'revoked':
        return 'bg-neutral-600'
      default:
        return 'bg-neutral-500'
    }
  }

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-neutral-900 border border-neutral-800 hover:border-neutral-700 text-xs text-neutral-200 transition select-none"
      >
        {activeNode ? (
          <>
            <span className={`w-2 h-2 rounded-full ${getStatusColor(activeNode.status)}`} />
            <Server className="w-3.5 h-3.5 text-brand-cyan" />
            <span className="font-semibold text-white max-w-[140px] truncate">{activeNode.name}</span>
          </>
        ) : (
          <>
            <Globe className="w-3.5 h-3.5 text-brand-cyan" />
            <span className="font-medium text-white">Todos os Servidores</span>
            <span className="px-1.5 py-0.2 rounded bg-neutral-800 text-[10px] text-neutral-400">
              {nodes.length} nós
            </span>
          </>
        )}
        <ChevronDown className="w-3.5 h-3.5 text-neutral-400 ml-1" />
      </button>

      {isOpen && (
        <div className="absolute left-0 mt-1.5 w-64 bg-neutral-900 border border-neutral-800 rounded-lg shadow-2xl py-1 z-50">
          <div className="px-3 py-1.5 text-[10px] uppercase font-semibold text-neutral-400 tracking-wider">
            Contexto de Servidor
          </div>

          <button
            onClick={() => {
              setActiveNodeId(null)
              setIsOpen(false)
            }}
            className="w-full flex items-center justify-between px-3 py-2 text-xs text-neutral-200 hover:bg-neutral-800/80 transition"
          >
            <div className="flex items-center gap-2">
              <Globe className="w-3.5 h-3.5 text-brand-cyan" />
              <span>Visão Global (Consolidada)</span>
            </div>
            {activeNodeId === null && <Check className="w-3.5 h-3.5 text-brand-cyan" />}
          </button>

          <div className="h-px bg-neutral-800 my-1" />

          <div className="max-h-60 overflow-y-auto">
            {nodes.map((node) => (
              <button
                key={node.id}
                onClick={() => {
                  setActiveNodeId(node.id)
                  setIsOpen(false)
                }}
                className="w-full flex items-center justify-between px-3 py-2 text-xs hover:bg-neutral-800/80 transition text-left"
              >
                <div className="flex items-center gap-2 truncate">
                  <span className={`w-2 h-2 rounded-full shrink-0 ${getStatusColor(node.status)}`} />
                  <div className="truncate">
                    <div className="font-medium text-neutral-200 truncate">{node.name}</div>
                    <div className="text-[10px] text-neutral-500 truncate">{node.group_name}</div>
                  </div>
                </div>
                {activeNodeId === node.id && <Check className="w-3.5 h-3.5 text-brand-cyan shrink-0 ml-2" />}
              </button>
            ))}
          </div>

          {onNavigateToServers && (
            <>
              <div className="h-px bg-neutral-800 my-1" />
              <button
                onClick={() => {
                  onNavigateToServers()
                  setIsOpen(false)
                }}
                className="w-full text-center px-3 py-1.5 text-xs text-brand-cyan hover:bg-neutral-800/50 transition font-medium"
              >
                Gerenciar Servidores...
              </button>
            </>
          )}
        </div>
      )}
    </div>
  )
}
