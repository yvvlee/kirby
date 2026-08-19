import { createContext, useContext } from 'react'

import type { LoginCredentials } from '@/api/auth'
import type { User } from '@/api/types'

export type AuthContextValue = {
  user: User | null
  authenticated: boolean
  systemAdmin: boolean
  initialized: boolean
  busy: boolean
  error: unknown
  login: (credentials: LoginCredentials) => Promise<void>
  logout: () => Promise<void>
  bootstrap: () => Promise<boolean>
  expire: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
