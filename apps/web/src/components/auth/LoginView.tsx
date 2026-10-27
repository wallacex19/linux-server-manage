import { useState, FormEvent } from 'react'
import { useAuth } from '../../context/AuthContext'
import { Server, Lock, User, AlertCircle, ShieldCheck } from 'lucide-react'

export function LoginView() {
  const { login, error } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) return

    setIsSubmitting(true)
    await login(username.trim(), password)
    setIsSubmitting(false)
  }

  return (
    <div className="min-h-screen bg-slate-950 flex flex-col justify-center items-center px-4">
      <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-8 shadow-xl">
        {/* Brand Icon & Heading */}
        <div className="text-center mb-8">
          <div className="inline-flex p-3 bg-indigo-500/10 border border-indigo-500/20 rounded-xl text-indigo-400 mb-3">
            <Server className="w-7 h-7" />
          </div>
          <h1 className="text-xl font-semibold text-white">Linux Server Manager</h1>
          <p className="text-xs text-slate-400 mt-1">Autenticação com contas do sistema Linux</p>
        </div>

        {/* Error notification */}
        {error && (
          <div className="mb-6 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 flex items-start gap-2.5 text-xs text-rose-300">
            <AlertCircle className="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        {/* Login Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              Usuário Linux
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-500">
                <User className="w-4 h-4" />
              </div>
              <input
                type="text"
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="ex.: admin ou seu usuário"
                className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-indigo-500 transition-colors"
                autoComplete="username"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              Senha do Sistema
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-500">
                <Lock className="w-4 h-4" />
              </div>
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••••••"
                className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-indigo-500 transition-colors"
                autoComplete="current-password"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full mt-2 py-2.5 px-4 bg-indigo-600 hover:bg-indigo-500 disabled:bg-indigo-800 text-white font-medium rounded-lg text-sm transition-colors shadow-sm flex items-center justify-center gap-2"
          >
            {isSubmitting ? (
              <span className="inline-block animate-pulse">Autenticando via PAM...</span>
            ) : (
              <span>Acessar Painel</span>
            )}
          </button>
        </form>

        {/* Security Notice */}
        <div className="mt-6 pt-5 border-t border-slate-800 flex items-start gap-2 text-[11px] text-slate-400">
          <ShieldCheck className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
          <span>
            Credenciais validadas diretamente no host Linux (PAM). Nenhuma senha é duplicada ou gravada em texto puro.
          </span>
        </div>
      </div>
    </div>
  )
}
