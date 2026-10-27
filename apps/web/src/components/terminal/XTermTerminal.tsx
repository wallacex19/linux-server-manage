import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { Wifi, WifiOff, RefreshCw } from 'lucide-react'

interface XTermTerminalProps {
  tmuxSession?: string
}

export function XTermTerminal({ tmuxSession }: XTermTerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [connError, setConnError] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)

  const connectWebSocket = () => {
    if (!terminalRef.current) return

    setConnError(null)

    // Inicializa xterm se ainda não existir
    if (!termRef.current) {
      const term = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily: 'Consolas, Monaco, "Courier New", monospace',
        theme: {
          background: '#020617', // slate-950
          foreground: '#f8fafc', // slate-50
          cursor: '#6366f1', // indigo-500
          selectionBackground: '#312e81',
          black: '#0f172a',
          red: '#f43f5e',
          green: '#10b981',
          yellow: '#f59e0b',
          blue: '#3b82f6',
          magenta: '#a855f7',
          cyan: '#06b6d4',
          white: '#f1f5f9',
        },
      })

      const fitAddon = new FitAddon()
      term.loadAddon(fitAddon)
      term.open(terminalRef.current)
      fitAddon.fit()

      termRef.current = term
      fitAddonRef.current = fitAddon
    }

    const term = termRef.current
    const fitAddon = fitAddonRef.current

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const sessionParam = tmuxSession ? `&tmux_session=${encodeURIComponent(tmuxSession)}` : ''
    const wsUrl = `${protocol}//${host}/api/v1/terminal/ws?client=web${sessionParam}`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setIsConnected(true)
      term.focus()
      if (fitAddon) {
        fitAddon.fit()
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }
      term.writeln('\x1b[38;5;81m[LSM Terminal] Sessão PTY inicializada.\x1b[0m')
    }

    ws.onmessage = (event) => {
      term.write(event.data)
    }

    ws.onerror = () => {
      setConnError('Erro na conexão WebSocket com o terminal')
      setIsConnected(false)
    }

    ws.onclose = () => {
      setIsConnected(false)
      term.writeln('\r\n\x1b[38;5;203m[LSM Terminal] Conexão encerrada.\x1b[0m\r\n')
    }

    const dataDisposable = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data)
      }
    })

    const handleResize = () => {
      if (fitAddon && ws.readyState === WebSocket.OPEN) {
        fitAddon.fit()
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }
    }

    window.addEventListener('resize', handleResize)

    return () => {
      dataDisposable.dispose()
      window.removeEventListener('resize', handleResize)
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
    }
  }

  useEffect(() => {
    const cleanup = connectWebSocket()
    return () => {
      if (cleanup) cleanup()
      if (termRef.current) {
        termRef.current.dispose()
        termRef.current = null
      }
    }
  }, [tmuxSession])

  return (
    <div className="flex flex-col h-full bg-slate-950 rounded-xl overflow-hidden border border-slate-800">
      {/* Terminal Top Info Bar */}
      <div className="h-9 px-4 bg-slate-900 border-b border-slate-800 flex items-center justify-between text-xs text-slate-400 shrink-0">
        <div className="flex items-center gap-2">
          {isConnected ? (
            <span className="flex items-center gap-1.5 text-emerald-400 font-medium">
              <Wifi className="w-3.5 h-3.5" />
              Conectado ao PTY
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-rose-400 font-medium">
              <WifiOff className="w-3.5 h-3.5" />
              Desconectado
            </span>
          )}
          {tmuxSession && (
            <span className="px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 text-[11px] font-mono">
              tmux: {tmuxSession}
            </span>
          )}
        </div>

        <button
          onClick={connectWebSocket}
          disabled={isConnected}
          className="flex items-center gap-1 hover:text-white disabled:opacity-40 transition-colors"
          title="Reconectar terminal"
        >
          <RefreshCw className="w-3 h-3" />
          <span>Reconectar</span>
        </button>
      </div>

      {connError && (
        <div className="p-2 bg-rose-500/10 text-rose-300 text-xs text-center border-b border-rose-500/20">
          {connError}
        </div>
      )}

      {/* Terminal DOM container */}
      <div
        ref={terminalRef}
        className="flex-1 w-full h-full p-2 bg-slate-950 overflow-hidden"
      />
    </div>
  )
}
