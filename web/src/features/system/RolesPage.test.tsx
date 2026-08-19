import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp } from 'antd'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as roleApi from '@/api/roles'
import RolesPage from './RolesPage'

vi.mock('@/api/roles', () => ({
  createRole: vi.fn(),
  deleteRole: vi.fn(),
  listPermissions: vi.fn(),
  listRoles: vi.fn(),
  updateRole: vi.fn(),
  updateRolePermissions: vi.fn(),
}))
vi.mock('@/auth/auth-state', () => ({
  useAuth: () => ({ systemAdmin: true }),
}))

const listRoles = vi.mocked(roleApi.listRoles)
const listPermissions = vi.mocked(roleApi.listPermissions)
const updateRolePermissions = vi.mocked(roleApi.updateRolePermissions)

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <AntdApp>
      <QueryClientProvider client={queryClient}><RolesPage /></QueryClientProvider>
    </AntdApp>,
  )
}

describe('RolesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listRoles.mockResolvedValue({
      list: [{
        id: 9,
        key: 'custom',
        name: '自定义角色',
        version: 1,
        permissions: [{ id: 701, key: 'backend:declared', name: '后端声明权限' }],
      }],
    })
    listPermissions.mockResolvedValue({
      list: [{ id: 701, key: 'backend:declared', name: '后端声明权限', description: '测试权限' }],
    })
    updateRolePermissions.mockResolvedValue({})
  })

  it('only displays and submits permissions declared by the backend', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: '配置权限' }))
    expect(screen.getByRole('checkbox', { name: /后端声明权限/ })).toBeChecked()
    await user.click(screen.getByRole('button', { name: '保存权限' }))

    await waitFor(() => expect(updateRolePermissions).toHaveBeenCalledWith(9, [701]))
  })

  it('keeps the page mounted and reports a forbidden response', async () => {
    listRoles.mockRejectedValue({ response: { status: 403 } })
    renderPage()

    expect(await screen.findByText('没有权限读取角色与权限。当前页面已保留。')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '角色与权限' })).toBeInTheDocument()
  })
})
