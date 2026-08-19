import {
  type PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'

import {
  login as loginRequest,
  logout as logoutRequest,
  refreshSession,
  type AuthenticationReply,
  type LoginCredentials,
} from '@/api/auth'
import type { User } from '@/api/types'
import { SESSION_CHANGED_DURING_REFRESH } from './refresh-coordinator'
import {
  clearAccessToken,
  getAccessToken,
  getAccessTokenSnapshot,
  onSessionExpired,
  startAccessTokenSession,
} from './token'
import { AuthContext, type AuthContextValue } from './auth-state'

function requireAuthenticationReply(reply: AuthenticationReply): AuthenticationReply {
  if (typeof reply.access_token !== 'string' || reply.access_token.length === 0) {
    throw new TypeError('authentication response does not contain access_token')
  }
  if (!reply.user || typeof reply.user !== 'object') {
    throw new TypeError('authentication response does not contain user')
  }
  return reply
}

function isAnonymousResponse(error: unknown): boolean {
  if (typeof error !== 'object' || error === null) return false
  if ('code' in error && error.code === SESSION_CHANGED_DURING_REFRESH) return true
  if (!('response' in error) || typeof error.response !== 'object' || error.response === null) {
    return false
  }
  if (!('status' in error.response)) return false
  return error.response.status === 401 || error.response.status === 403
}

export function AuthProvider({ children }: PropsWithChildren) {
  const [user, setUser] = useState<User | null>(null)
  const [initialized, setInitialized] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const bootstrapOperation = useRef<Promise<boolean> | null>(null)

  const expire = useCallback(() => {
    const current = getAccessTokenSnapshot()
    if (current.sessionActive || current.refreshAllowed) {
      clearAccessToken()
    }
    setUser(null)
    setInitialized(true)
  }, [])

  const login = useCallback(async (credentials: LoginCredentials) => {
    setBusy(true)
    setError(null)
    try {
      const reply = requireAuthenticationReply(await loginRequest(credentials))
      startAccessTokenSession(reply.access_token)
      setUser(reply.user)
      setInitialized(true)
    } catch (error: unknown) {
      clearAccessToken()
      setUser(null)
      setError(error)
      throw error
    } finally {
      setBusy(false)
    }
  }, [])

  const logout = useCallback(async () => {
    const accessToken = getAccessToken()
    const request = logoutRequest(accessToken)
    clearAccessToken()
    setUser(null)
    setInitialized(true)
    await request
  }, [])

  const bootstrap = useCallback((): Promise<boolean> => {
    if (initialized) {
      return Promise.resolve(Boolean(user && getAccessToken()))
    }
    if (bootstrapOperation.current) {
      return bootstrapOperation.current
    }

    const startedSessionGeneration = getAccessTokenSnapshot().sessionGeneration
    let refreshedSessionGeneration: number | null = null
    const operation = (async () => {
      try {
        const reply = requireAuthenticationReply(await refreshSession())
        refreshedSessionGeneration = getAccessTokenSnapshot().sessionGeneration
        if (getAccessToken() !== reply.access_token) {
          throw new Error('refresh reply does not match the active access token')
        }
        setUser(reply.user)
        setInitialized(true)
        return true
      } catch (error: unknown) {
        const current = getAccessTokenSnapshot()
        if (
          current.sessionGeneration !== startedSessionGeneration &&
          current.sessionGeneration !== refreshedSessionGeneration &&
          current.sessionActive
        ) {
          return Boolean(getAccessToken())
        }
        if (
          (current.sessionGeneration === startedSessionGeneration ||
            current.sessionGeneration === refreshedSessionGeneration) &&
          (current.sessionActive || current.refreshAllowed)
        ) {
          clearAccessToken()
        }
        setUser(null)
        setInitialized(true)
        if (isAnonymousResponse(error)) return false
        throw error
      } finally {
        bootstrapOperation.current = null
      }
    })()
    bootstrapOperation.current = operation
    return operation
  }, [initialized, user])

  useEffect(() => {
    const unsubscribe = onSessionExpired(() => expire())
    void bootstrap().catch(setError)
    return unsubscribe
  }, [bootstrap, expire])

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      authenticated: Boolean(user && getAccessToken()),
      systemAdmin: Boolean(user?.is_system_admin),
      initialized,
      busy,
      error,
      login,
      logout,
      bootstrap,
      expire,
    }),
    [bootstrap, busy, error, expire, initialized, login, logout, user],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
