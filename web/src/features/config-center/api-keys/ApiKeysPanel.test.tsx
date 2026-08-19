import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp } from 'antd'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import * as api from '@/api/api-keys'
import { queryKeys } from '@/app/query-keys'
import { EnvironmentContext, type EnvironmentContextValue } from '@/auth/environment-state'
import ApiKeysPanel from './ApiKeysPanel'

vi.mock('@/api/api-keys', () => ({
  createProjectApiKey: vi.fn(),
  listProjectApiKeys: vi.fn(),
  revokeProjectApiKey: vi.fn(),
  rotateProjectApiKey: vi.fn(),
}))

const environment = { id: 11, key: 'east', name: 'East', enabled: true, version: 1 }
const environmentValue: EnvironmentContextValue = {
  available: [environment],
  current: environment,
  currentId: 11,
  permissions: ['project:api_key:read', 'project:api_key:manage'],
  switching: false,
  initialized: true,
  error: null,
  hasPermission: (permission) => ['project:api_key:read', 'project:api_key:manage'].includes(permission),
  loadAvailable: async () => [environment],
  select: async () => environment,
  resetScope: async () => undefined,
}

describe('ApiKeysPanel', () => {
  it('keeps a one-time secret out of query cache and clears it after acknowledgment', async () => {
    vi.mocked(api.listProjectApiKeys).mockResolvedValue({ list: [] })
    vi.mocked(api.createProjectApiKey).mockResolvedValue({ apiKey: { id: 5 }, secret: 'secret-visible-once' })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const user = userEvent.setup()
    render(
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <EnvironmentContext.Provider value={environmentValue}>
            <ApiKeysPanel open projectId={7} onClose={vi.fn()} />
          </EnvironmentContext.Provider>
        </QueryClientProvider>
      </AntdApp>,
    )

    await user.click(await screen.findByRole('button', { name: /创建 API Key/ }))
    const createDialog = screen.getAllByRole('dialog')[1]
    if (!createDialog) throw new Error('创建 API Key 对话框没有渲染')
    await user.type(within(createDialog).getByRole('textbox', { name: '名称' }), 'production')
    await user.click(within(createDialog).getByRole('button', { name: /创\s*建/ }))

    const secretText = await screen.findByText('secret-visible-once')
    expect(secretText).toBeInTheDocument()
    expect(JSON.stringify(queryClient.getQueryData(queryKeys.apiKeys(11, 7)))).not.toContain('secret-visible-once')
    const secretDialog = secretText.closest<HTMLElement>('[role="dialog"]')
    if (!secretDialog) throw new Error('Secret 对话框没有渲染')
    await user.click(within(secretDialog).getByRole('checkbox'))
    const clearButton = within(secretDialog).getByRole('button', { name: '确认并清除' })
    await waitFor(() => expect(clearButton).toBeEnabled())
    fireEvent.click(clearButton)
    await waitFor(() => expect(screen.queryByText('secret-visible-once')).not.toBeInTheDocument())
  })
})
