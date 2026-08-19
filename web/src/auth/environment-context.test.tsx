import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const environmentApi = vi.hoisted(() => ({
  getMyPermissions: vi.fn(),
  listEnvironments: vi.fn(),
}))
vi.mock('@/api/environments', () => environmentApi)

import { createQueryClient } from '@/app/query-client'
import { queryKeys } from '@/app/query-keys'
import { AuthContext } from './auth-state'
import { EnvironmentProvider } from './environment-context'
import { useEnvironment } from './environment-state'
import { clearAccessToken, startAccessTokenSession } from './token'

const environments = [
  { id: 1, key: 'one', name: '环境一', enabled: true, version: 1 },
  { id: 2, key: 'two', name: '环境二', enabled: true, version: 1 },
]

function deferred<T>() {
  let resolvePromise: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve
  })
  return { promise, resolve: (value: T) => resolvePromise?.(value) }
}

function EnvironmentProbe() {
  const environment = useEnvironment()
  return (
    <div>
      <output aria-label="可用环境">{environment.available.length}</output>
      <output aria-label="当前环境">{environment.currentId ?? 'none'}</output>
      <output aria-label="权限">{environment.permissions.join(',')}</output>
      <button type="button" onClick={() => void environment.select(2)}>
        切换环境二
      </button>
    </div>
  )
}

const authenticatedValue = {
  user: {
    id: 1,
    username: 'admin',
    is_system_admin: true,
    version: 1,
  },
  authenticated: true,
  systemAdmin: true,
  initialized: true,
  busy: false,
  error: null,
  login: vi.fn(),
  logout: vi.fn(),
  bootstrap: vi.fn(async () => true),
  expire: vi.fn(),
}

beforeEach(() => {
  vi.clearAllMocks()
  startAccessTokenSession('environment-token')
  environmentApi.listEnvironments.mockResolvedValue({ list: environments })
})

afterEach(() => {
  clearAccessToken()
})

describe('EnvironmentProvider', () => {
  it('keeps the latest switch and removes the previous scope cache', async () => {
    const first = deferred<{ permissions: string[] }>()
    const second = deferred<{ permissions: string[] }>()
    environmentApi.getMyPermissions.mockImplementation((environmentId: number) =>
      environmentId === 1 ? first.promise : second.promise,
    )
    const queryClient = createQueryClient()
    queryClient.setQueryData(queryKeys.projects(1), { list: ['old'] })
    const actor = userEvent.setup()

    render(
      <QueryClientProvider client={queryClient}>
        <AuthContext.Provider value={authenticatedValue}>
          <EnvironmentProvider>
            <EnvironmentProbe />
          </EnvironmentProvider>
        </AuthContext.Provider>
      </QueryClientProvider>,
    )

    await waitFor(() => {
      expect(screen.getByLabelText('可用环境')).toHaveTextContent('2')
    })
    await actor.click(screen.getByRole('button', { name: '切换环境二' }))
    second.resolve({ permissions: ['project:write'] })
    await waitFor(() => {
      expect(screen.getByLabelText('当前环境')).toHaveTextContent('2')
    })
    first.resolve({ permissions: ['stale:permission'] })
    await waitFor(() => {
      expect(screen.getByLabelText('权限')).toHaveTextContent('project:write')
    })

    expect(queryClient.getQueryData(queryKeys.projects(1))).toBeUndefined()
  })
})
