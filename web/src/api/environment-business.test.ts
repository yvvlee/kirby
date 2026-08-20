import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({ post: vi.fn(), get: vi.fn(), put: vi.fn() }))
vi.mock('./client', () => ({ default: client }))

import {
  createProject,
  getProject,
  listProjects,
  updateProject,
} from './projects'

beforeEach(() => {
  client.post.mockReset()
  client.get.mockReset()
  client.put.mockReset()
  client.post.mockResolvedValue({ data: { ok: true } })
  client.get.mockResolvedValue({ data: { list: [] } })
  client.put.mockResolvedValue({ data: { ok: true } })
})

describe('environment business API contracts', () => {
  it('creates a project at the global project endpoint', async () => {
    await createProject(null, { key: 'Demo' })
    expect(client.post).toHaveBeenCalledWith('/admin/projects', { key: 'Demo' })
  })

  it('updates and lists projects at global endpoints', async () => {
    await updateProject(null, { id: 2 })
    await listProjects(null, { keyword: 'd' })
    await getProject(null, 2)
    expect(client.put).toHaveBeenCalledWith('/admin/projects/2', { id: 2 })
    expect(client.get).toHaveBeenCalledWith('/admin/projects', { params: { keyword: 'd' } })
    expect(client.get).toHaveBeenCalledWith('/admin/projects/2')
  })

  it('rejects an invalid environment before sending a request', async () => {
    await expect(listProjects(null)).resolves.toEqual({ list: [] })
  })

})
