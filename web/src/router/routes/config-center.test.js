import { describe, expect, it, vi } from 'vitest'

vi.mock('@/features/config-center/configs/ConfigDetailPage.vue', () => ({
  default: { name: 'ConfigDetailPage' },
}))
vi.mock('@/features/config-center/configs/ConfigsPage.vue', () => ({
  default: { name: 'ConfigsPage' },
}))
vi.mock('@/features/config-center/projects/ProjectsPage.vue', () => ({
  default: { name: 'ProjectsPage' },
}))

import configCenterRoutes from './config-center'
import { createRouter } from '../index'

describe('config center routes', () => {
  it('uses the frozen standalone paths', () => {
    expect(configCenterRoutes.map((route) => route.path)).toEqual([
      'projects',
      'projects/:projectId/configs',
      'projects/:projectId/configs/:configId',
    ])
    expect(configCenterRoutes.map((route) => route.name)).toEqual([
      'project-list',
      'project-configs',
      'config-detail',
    ])
  })

  it('converts route IDs to positive numbers and fails on invalid IDs', () => {
    expect(configCenterRoutes[1].props({ params: { projectId: '12' } })).toEqual({
      projectId: 12,
    })
    expect(
      configCenterRoutes[2].props({
        params: { projectId: '12', configId: '34' },
      }),
    ).toEqual({ projectId: 12, configId: 34 })
    expect(() =>
      configCenterRoutes[1].props({ params: { projectId: '0' } }),
    ).toThrow('projectId must be a positive integer')
  })

  it('keeps read permissions on every page', () => {
    expect(configCenterRoutes.map((route) => route.meta.permission)).toEqual([
      'project:read',
      'config:read',
      'config:read',
    ])
  })

  it('is mounted below the authenticated application layout', () => {
    const store = {
      dispatch: vi.fn(),
      getters: {
        'session/authenticated': true,
        'session/systemAdmin': false,
        'environment/hasPermission': () => true,
      },
      state: {
        environment: { currentId: 1 },
      },
    }
    const router = createRouter(store, { mode: 'abstract' })
    const match = router.match('/projects/12/configs/34')

    expect(match.name).toBe('config-detail')
    expect(match.matched).toHaveLength(2)
    expect(match.matched[0].meta.requiresAuth).toBe(true)
    expect(match.matched[1].meta.permission).toBe('config:read')
  })
})
