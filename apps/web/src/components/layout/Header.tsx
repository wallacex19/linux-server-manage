import { useAuth } from '../../context/AuthContext'
import { Shield, ShieldAlert, LogOut, Terminal } from 'lucide-react'
import { ServerSelector } from './ServerSelector'

interface HeaderProps {
  currentViewLabel: string
  onNavigateToServers?: () => void
}

export function Header({ currentViewLabel, onNavigateToServers }: HeaderProps) {
  const { user, logout } = useAuth()

  return (
    <header className="h-16 bg-slate-900/60 backdrop-blur border-b border-slate-800 px-6 flex items-center justify-between shrink-0">
      {/* View Title & Server Selector */}
      <div className="flex items-center gap-4">
        <h1 className="text-base font-semibold text-white">{currentViewLabel}</h1>
        <ServerSelector onNavigateToServers={onNavigateToServers} />
      </div>

      {/* User and Session Controls */}
      <div className="flex items-center gap-4">
        {user ? (
          <div className="flex items-center gap-3">
            {/* Identity Info */}
            <div className="text-right hidden md:block">
              <div className="flex items-center justify-end gap-1.5">
                <span className="text-xs font-semibold text-slate-200">{user.username}</span>
                {user.is_admin ? (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 font-medium">
                    <Shield className="w-2.5 h-2.5" />
                    Admin
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-slate-800 text-slate-400 border border-slate-700 font-medium">
                    <Terminal className="w-2.5 h-2.5" />
                    Usuário
                  </span>
                )}
              </div>
              <p className="text-[10px] text-slate-400">
                UID: {user.uid} · GID: {user.gid} · Home: {user.home_dir}
              </p>
            </div>

            {/* Logout Button */}
            <button
              onClick={() => logout()}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium text-slate-300 hover:text-white bg-slate-800 hover:bg-rose-950/40 hover:text-rose-300 border border-slate-700 hover:border-rose-800/40 transition-colors"
              title="Encerrar sessão no servidor"
            >
              <LogOut className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Sair</span>
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-xs text-amber-400 bg-amber-500/10 border border-amber-500/20 px-3 py-1 rounded-lg">
            <ShieldAlert className="w-4 h-4" />
            <span>Não autenticado</span>
          </div>
        )}
      </div>
    </header>
  )
}
