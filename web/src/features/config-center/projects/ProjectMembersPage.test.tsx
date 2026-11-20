import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, expect, it, vi } from 'vitest'
import { getProject } from '@/api/projects'
import * as members from '@/api/project-members'
import * as invitations from '@/api/project-invitations'
import { getRegistrationStatus } from '@/api/auth'
import ProjectMembersPage from './ProjectMembersPage'

vi.mock('@/auth/auth-state', () => ({ useAuth: () => ({ user: { id: 1 } }) }))
vi.mock('@/api/projects', () => ({ getProject: vi.fn() }))
vi.mock('@/api/auth', () => ({ getRegistrationStatus: vi.fn() }))
vi.mock('@/api/project-members', () => ({
  listProjectMembers: vi.fn(),
  searchProjectUsers: vi.fn(),
  addProjectMember: vi.fn(),
  updateProjectMemberRole: vi.fn(),
  removeProjectMember: vi.fn(),
}))
vi.mock('@/api/project-invitations', () => ({
  createProjectInvitation: vi.fn(),
  revokeProjectInvitation: vi.fn(),
}))

function renderPage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  render(<QueryClientProvider client={client}>
    <MemoryRouter initialEntries={['/projects/7/members']}>
      <Routes><Route path="/projects/:projectId/members" element={<ProjectMembersPage />} /></Routes>
    </MemoryRouter>
  </QueryClientProvider>)
}

function membership(role: string) {
  vi.mocked(members.listProjectMembers).mockResolvedValue({
    list: [{
      role,
      user: {
        id: 1,
        username: 'alice',
        nickname: 'Alice',
        is_system_admin: false,
        version: 0,
      },
    }],
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(getProject).mockResolvedValue({ project: { id: 7, key: 'Demo', name: '示例项目' } } as never)
  membership('OWNER')
  vi.mocked(members.searchProjectUsers).mockResolvedValue({ list: [] })
  vi.mocked(getRegistrationStatus).mockResolvedValue(false)
})

it('allows existing-user invitations but hides link generation while registration is closed', async () => {
  renderPage()
  await screen.findByRole('button', { name: '添加成员' })
  expect(screen.queryByRole('button', { name: '生成邀请链接' })).not.toBeInTheDocument()
})

it('does not expose member management to a Guest even when registration is enabled', async () => {
  membership('GUEST')
  vi.mocked(getRegistrationStatus).mockResolvedValue(true)
  renderPage()
  await screen.findByText('Alice')
  expect(screen.queryByRole('button', { name: '添加成员' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '生成邀请链接' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '移除' })).not.toBeInTheDocument()
})

it('generates a link without adding a member and supports revocation', async () => {
  vi.mocked(getRegistrationStatus).mockResolvedValue(true)
  vi.mocked(invitations.createProjectInvitation).mockResolvedValue({
    token: 'single-use-token',
    project_id: 7,
    role: 'GUEST',
    expires_at: '2026-11-19T12:00:00Z',
  })
  vi.mocked(invitations.revokeProjectInvitation).mockResolvedValue()
  const user = userEvent.setup()
  renderPage()
  await user.click(await screen.findByRole('button', { name: '生成邀请链接' }))
  await screen.findByText(/\/invitations\/single-use-token/)
  expect(invitations.createProjectInvitation).toHaveBeenCalledWith('7', 'GUEST')
  expect(members.addProjectMember).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: '撤销链接' }))
  await waitFor(() => expect(screen.queryByRole('button', { name: '撤销链接' })).not.toBeInTheDocument())
  expect(invitations.revokeProjectInvitation).toHaveBeenCalledWith('single-use-token')
})

it('loads users initially and searches from the same selector', async () => {
  const user = userEvent.setup()
  renderPage()
  const picker = await screen.findByRole('combobox', { name: '选择已有用户' })
  await waitFor(() => expect(members.searchProjectUsers).toHaveBeenCalledWith('7', ''))
  expect(screen.getByRole('link', { name: '示例项目' })).toHaveAttribute('href', '/projects/7/configs')
  await user.type(picker, 'alice@example.com')
  await waitFor(() => expect(members.searchProjectUsers).toHaveBeenLastCalledWith('7', 'alice@example.com'))
})
