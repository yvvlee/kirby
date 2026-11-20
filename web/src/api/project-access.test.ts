import { expect, it, vi } from 'vitest'
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }))
vi.mock('./client', () => ({ default: client }))
import { searchProjectUsers, addProjectMember, updateProjectMemberRole, removeProjectMember } from './project-members'
import { updateEnvironmentPermissionMatrix } from './environment-permissions'
it('keeps member operations in the selected project and sends fixed roles', async () => {
  client.get.mockResolvedValue({ data: { list: [] } })
  client.post.mockResolvedValue({ data: {} })
  client.put.mockResolvedValue({ data: {} })
  await searchProjectUsers(3, 'alice')
  await addProjectMember(3, 7, 'MEMBER')
  await updateProjectMemberRole(3, 7, 'GUEST')
  await removeProjectMember(3, 7)
  expect(client.get).toHaveBeenCalledWith('/admin/projects/3/users', { params: { keyword: 'alice' } })
  expect(client.post).toHaveBeenCalledWith('/admin/projects/3/members', { project_id: 3, user_id: 7, role: 'MEMBER' })
  expect(client.put).toHaveBeenCalledWith('/admin/projects/3/members/7', { project_id: 3, user_id: 7, role: 'GUEST' })
  expect(client.delete).toHaveBeenCalledWith('/admin/projects/3/members/7')
})
it('writes a permission matrix only to its selected environment', async () => {
  client.put.mockResolvedValue({ data: {} })
  const roles = [{ role: 'MEMBER', permissions: ['READ', 'API_KEY_MANAGE'] }]
  await updateEnvironmentPermissionMatrix(9, roles)
  expect(client.put).toHaveBeenCalledWith('/admin/environments/9/permissions', { environment_id: 9, roles })
})
