import { render, screen, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const authApi = vi.hoisted(() => ({
  login: vi.fn(),
  logout: vi.fn(),
  refreshSession: vi.fn(),
}))
vi.mock('@/api/auth', () => authApi)

import { AuthProvider } from './auth-context'
import { useAuth } from './auth-state'
import {
  clearAccessToken,
  getAccessToken,
  startAccessTokenSession,
} from './token'

const user = {
  id: 1,
  username: 'admin',
  is_system_admin: true,
  version: 1,
}

function deferred<T>() {
  let resolvePromise: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve
  })
  return { promise, resolve: (value: T) => resolvePromise?.(value) }
}

function AuthProbe() {
  const auth = useAuth()
  return (
    <div>
      <output aria-label="认证状态">
        {auth.initialized ? auth.user?.username ?? 'anonymous' : 'loading'}
      </output>
      <button
        type="button"
        onClick={() => void auth.login({ username: 'admin', password: 'secret' })}
      >
        登录
      </button>
    </div>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  clearAccessToken()
})

describe('AuthProvider', () => {
  it('deduplicates bootstrap in React Strict Mode', async () => {
    const refresh = deferred<{
      access_token: string
      user: typeof user
    }>()
    authApi.refreshSession.mockImplementation(async () => {
      const reply = await refresh.promise
      startAccessTokenSession(reply.access_token)
      return reply
    })

    render(
      <StrictMode>
        <AuthProvider>
          <AuthProbe />
        </AuthProvider>
      </StrictMode>,
    )

    expect(authApi.refreshSession).toHaveBeenCalledTimes(1)
    refresh.resolve({ access_token: 'bootstrap-token', user })
    await waitFor(() => {
      expect(screen.getByLabelText('认证状态')).toHaveTextContent('admin')
    })
    expect(authApi.refreshSession).toHaveBeenCalledTimes(1)
    expect(getAccessToken()).toBe('bootstrap-token')
  })
})
