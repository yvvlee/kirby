import Vue from 'vue'
import Vuex from 'vuex'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  listConfigs: vi.fn(),
  listEnums: vi.fn(),
  listModels: vi.fn(),
  listProjects: vi.fn(),
  listSnapshots: vi.fn(),
}))

const environmentApi = vi.hoisted(() => ({
  getMyPermissions: vi.fn(),
  listEnvironments: vi.fn(),
}))

vi.mock('@/api/configs', () => ({ listConfigs: api.listConfigs }))
vi.mock('@/api/enums', () => ({ listEnums: api.listEnums }))
vi.mock('@/api/models', () => ({ listModels: api.listModels }))
vi.mock('@/api/projects', () => ({ listProjects: api.listProjects }))
vi.mock('@/api/snapshots', () => ({ listSnapshots: api.listSnapshots }))
vi.mock('@/api/environments', () => environmentApi)

import { clearEnvironmentScope } from '@/auth/environment-scope'
import { createStore as createRootStore } from '@/store'
import configCenter from './config-center'

Vue.use(Vuex)

function createStore() {
  return new Vuex.Store({
    strict: true,
    modules: { configCenter },
  })
}

function deferred() {
  let resolve
  const promise = new Promise((done) => {
    resolve = done
  })
  return { promise, resolve }
}

describe('config center store', () => {
  let store

  beforeEach(() => {
    vi.clearAllMocks()
    store = createStore()
    api.listProjects.mockImplementation((environmentId) =>
      Promise.resolve({ list: [{ id: environmentId, environment_id: environmentId }] }),
    )
    api.listConfigs.mockImplementation((environmentId, filter) =>
      Promise.resolve({
        list: [{ id: environmentId * 100 + filter.project_id }],
      }),
    )
    api.listModels.mockImplementation((_environmentId, filter) =>
      Promise.resolve({ list: [{ id: filter.config_id }] }),
    )
    api.listEnums.mockResolvedValue({ list: [] })
    api.listSnapshots.mockResolvedValue({
      page: { page: 1, limit: 20, total: 0 },
      list: [],
    })
    environmentApi.listEnvironments.mockResolvedValue({
      list: [
        { id: 11, key: 'east', name: 'East', enabled: true },
        { id: 22, key: 'west', name: 'West', enabled: true },
      ],
    })
    environmentApi.getMyPermissions.mockResolvedValue({ permissions: [] })
  })

  it('isolates the same project ID in different environments', async () => {
    await store.dispatch('configCenter/loadConfigs', {
      environmentId: 11,
      projectId: 7,
    })
    await store.dispatch('configCenter/loadConfigs', {
      environmentId: 22,
      projectId: 7,
    })

    expect(store.getters['configCenter/configs'](11, 7)).toEqual([
      { id: 1107 },
    ])
    expect(store.getters['configCenter/configs'](22, 7)).toEqual([
      { id: 2207 },
    ])

    await store.dispatch('configCenter/loadConfigs', {
      environmentId: 11,
      projectId: 7,
    })
    expect(api.listConfigs).toHaveBeenCalledTimes(2)
  })

  it('is registered in the root store and responds to environment cleanup', async () => {
    const rootStore = createRootStore()
    await rootStore.dispatch('environment/loadAvailable')
    await rootStore.dispatch('configCenter/loadProjects', {
      environmentId: 11,
    })

    expect(rootStore.getters['configCenter/projects'](11)).toHaveLength(1)
    await rootStore.dispatch('environment/select', 22)
    expect(rootStore.getters['configCenter/projects'](11)).toEqual([])
  })

  it('includes config ID without dropping environment and project scope', async () => {
    await store.dispatch('configCenter/loadModels', {
      environmentId: 11,
      projectId: 7,
      configId: 31,
    })
    await store.dispatch('configCenter/loadModels', {
      environmentId: 11,
      projectId: 7,
      configId: 32,
    })

    expect(store.getters['configCenter/models'](11, 7, 31)).toEqual([
      { id: 31 },
    ])
    expect(store.getters['configCenter/models'](11, 7, 32)).toEqual([
      { id: 32 },
    ])
  })

  it('clears only the old environment through the shared cleanup protocol', async () => {
    await store.dispatch('configCenter/loadProjects', { environmentId: 11 })
    await store.dispatch('configCenter/loadProjects', { environmentId: 22 })

    await clearEnvironmentScope({
      fromEnvironmentId: 11,
      toEnvironmentId: 22,
    })

    expect(store.getters['configCenter/projects'](11)).toEqual([])
    expect(store.getters['configCenter/projects'](22)).toEqual([
      { id: 22, environment_id: 22 },
    ])
  })

  it('does not restore an old environment cache from an in-flight response', async () => {
    const pending = deferred()
    api.listProjects.mockReturnValueOnce(pending.promise)
    const loading = store.dispatch('configCenter/loadProjects', {
      environmentId: 11,
    })

    await clearEnvironmentScope({
      fromEnvironmentId: 11,
      toEnvironmentId: 22,
    })
    pending.resolve({ list: [{ id: 11, environment_id: 11 }] })
    await loading

    expect(store.getters['configCenter/projects'](11)).toEqual([])
  })

  it('fails immediately when a list response does not match the contract', async () => {
    api.listProjects.mockResolvedValueOnce({ data: { list: [] } })

    await expect(
      store.dispatch('configCenter/loadProjects', { environmentId: 11 }),
    ).rejects.toThrow('project list response does not contain list')
  })

  it('forces project and config IDs from action scope into requests', async () => {
    await store.dispatch('configCenter/loadModels', {
      environmentId: 11,
      projectId: 7,
      configId: 31,
      filter: { project_id: 99, config_id: 98 },
    })

    expect(api.listModels).toHaveBeenCalledWith(11, {
      project_id: 7,
      config_id: 31,
    })
  })
})
