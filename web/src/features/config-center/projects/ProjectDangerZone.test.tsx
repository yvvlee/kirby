import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { expect, it, vi } from 'vitest'
import { deleteProject } from '@/api/projects'
import ProjectDangerZone from './ProjectDangerZone'

vi.mock('@/api/projects', () => ({ deleteProject: vi.fn() }))

it('requires the exact saved name and resets confirmation when reopened', async () => {
  vi.mocked(deleteProject).mockResolvedValue()
  const user = userEvent.setup()
  render(<QueryClientProvider client={new QueryClient()}>
    <MemoryRouter><ProjectDangerZone projectId="7" projectName="Demo" /></MemoryRouter>
  </QueryClientProvider>)
  await user.click(screen.getByRole('button', { name: '删除项目' }))
  const confirm = await screen.findByRole('button', { name: '永久删除' })
  expect(confirm).toBeDisabled()
  await user.type(screen.getByRole('textbox', { name: '确认项目名称' }), 'demo')
  expect(confirm).toBeDisabled()
  await user.clear(screen.getByRole('textbox', { name: '确认项目名称' }))
  await user.type(screen.getByRole('textbox', { name: '确认项目名称' }), 'Demo')
  expect(confirm).toBeEnabled()
  await user.click(screen.getByRole('button', { name: /取\s*消|Cancel/ }))
  await user.click(screen.getByRole('button', { name: '删除项目' }))
  expect(screen.getByRole('button', { name: '永久删除' })).toBeDisabled()
  expect(deleteProject).not.toHaveBeenCalled()
  await user.type(screen.getByRole('textbox', { name: '确认项目名称' }), 'Demo')
  await user.click(screen.getByRole('button', { name: '永久删除' }))
  await waitFor(() => expect(deleteProject).toHaveBeenCalledWith('7'))
})
