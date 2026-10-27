import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { Node } from '../types/nodes'

interface ServerContextType {
  nodes: Node[]
  activeNodeId: string | null // null = 'Todos os Servidores' (Visão Global)
  activeNode: Node | null
  setActiveNodeId: (id: string | null) => void
  isLoading: boolean
  refreshNodes: () => Promise<void>
}

const ServerContext = createContext<ServerContextType | undefined>(undefined)

export function ServerProvider({ children }: { children: ReactNode }) {
  const [nodes, setNodes] = useState<Node[]>([])
  const [activeNodeId, setActiveNodeId] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const fetchNodes = async () => {
    try {
      const res = await fetch('/api/v1/nodes', { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        setNodes(Array.isArray(data) ? data : [])
      }
    } catch {
      // Ignora erro silencioso de polling
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchNodes()
    const interval = setInterval(fetchNodes, 5000)
    return () => clearInterval(interval)
  }, [])

  const activeNode = activeNodeId ? nodes.find((n) => n.id === activeNodeId) || null : null

  return (
    <ServerContext.Provider
      value={{
        nodes,
        activeNodeId,
        activeNode,
        setActiveNodeId,
        isLoading,
        refreshNodes: fetchNodes,
      }}
    >
      {children}
    </ServerContext.Provider>
  )
}

export function useServers() {
  const context = useContext(ServerContext)
  if (!context) {
    throw new Error('useServers deve ser utilizado dentro de um ServerProvider')
  }
  return context
}
