import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { LinuxUserIdentity } from '../types/auth'

interface AuthContextType {
  user: LinuxUserIdentity | null
  isLoading: boolean
  error: string | null
  login: (username: string, password: string) => Promise<boolean>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<LinuxUserIdentity | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Verifica se o usuário já possui sessão ativa
  const checkSession = async () => {
    try {
      const res = await fetch('/api/v1/auth/me', {
        credentials: 'include',
      })
      if (res.ok) {
        const userData: LinuxUserIdentity = await res.json()
        setUser(userData)
      } else {
        setUser(null)
      }
    } catch {
      setUser(null)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    checkSession()
  }, [])

  const login = async (username: string, password: string): Promise<boolean> => {
    setError(null)
    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ username, password }),
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        setError(data.error || 'Credenciais Linux inválidas.')
        return false
      }

      const data = await res.json()
      setUser(data.user)
      return true
    } catch {
      setError('Falha de conexão com o servidor.')
      return false
    }
  }

  const logout = async () => {
    try {
      await fetch('/api/v1/auth/logout', {
        method: 'POST',
        credentials: 'include',
      })
    } finally {
      setUser(null)
    }
  }

  return (
    <AuthContext.Provider value={{ user, isLoading, error, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth deve ser utilizado dentro de AuthProvider')
  }
  return context
}
