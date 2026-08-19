import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('./client', () => ({ default: client }))

import {
  createConfig,
  deleteConfig,
  getConfig,
  listConfigs,
  updateConfig,
  updateConfigValue,
} from './configs'
import { createEnum, deleteEnum, listEnums, updateEnum } from './enums'
import { createModel, deleteModel, listModels, updateModel } from './models'
import {
  createProject,
  getProject,
  listProjects,
  updateProject,
} from './projects'
import {
  createSnapshot,
  deleteSnapshot,
  getCurrentSnapshot,
  getReleasedSnapshot,
  getSnapshot,
  listSnapshots,
  loadSnapshot,
  previewCreatingSnapshot,
} from './snapshots'
import type { ApiObject } from './types'

type ContractCase = readonly [
  name: string,
  request: () => Promise<unknown>,
  suffix: string,
  body: ApiObject,
]

const cases: ContractCase[] = [
  ['create project', () => createProject(12, { key: 'Demo' }), 'project/create', { key: 'Demo' }],
  ['update project', () => updateProject(12, { id: 2 }), 'project/update', { id: 2 }],
  ['list projects', () => listProjects(12, { keyword: 'd' }), 'project/list', { keyword: 'd' }],
  ['project detail', () => getProject(12, 2), 'project/detail', { id: 2 }],
  ['create config', () => createConfig(12, { project_id: 2 }), 'config/create', { project_id: 2 }],
  ['update config', () => updateConfig(12, { id: 3 }), 'config/update', { id: 3 }],
  ['update config value', () => updateConfigValue(12, { id: 3, value: '{}' }), 'config/updateValue', { id: 3, value: '{}' }],
  ['list configs', () => listConfigs(12, { project_id: 2 }), 'config/list', { project_id: 2 }],
  ['config detail', () => getConfig(12, 3), 'config/detail', { id: 3 }],
  ['delete config', () => deleteConfig(12, 3), 'config/delete', { id: 3 }],
  ['create model', () => createModel(12, { config_id: 3 }), 'structure/create', { config_id: 3 }],
  ['update model', () => updateModel(12, { id: 4 }), 'structure/update', { id: 4 }],
  ['list models', () => listModels(12, { project_id: 2 }), 'structure/list', { project_id: 2 }],
  ['delete model', () => deleteModel(12, 4), 'structure/delete', { id: 4 }],
  ['create enum', () => createEnum(12, { config_id: 3 }), 'enum/create', { config_id: 3 }],
  ['update enum', () => updateEnum(12, { id: 5 }), 'enum/update', { id: 5 }],
  ['list enums', () => listEnums(12, { project_id: 2 }), 'enum/list', { project_id: 2 }],
  ['delete enum', () => deleteEnum(12, 5), 'enum/delete', { id: 5 }],
  ['create snapshot', () => createSnapshot(12, { config_id: 3 }), 'snapshot/create', { config_id: 3 }],
  ['preview snapshot', () => previewCreatingSnapshot(12, 3), 'snapshot/previewCreating', { config_id: 3 }],
  ['delete snapshot', () => deleteSnapshot(12, 6), 'snapshot/delete', { id: 6 }],
  ['snapshot detail', () => getSnapshot(12, 6), 'snapshot/detail', { id: 6 }],
  ['load snapshot', () => loadSnapshot(12, 6, 3), 'snapshot/load', { id: 6, config_id: 3 }],
  ['current snapshot', () => getCurrentSnapshot(12, 3), 'snapshot/current', { config_id: 3 }],
  ['released snapshot', () => getReleasedSnapshot(12, 3), 'snapshot/released', { config_id: 3 }],
  ['list snapshots', () => listSnapshots(12, { config_id: 3 }), 'snapshot/list', { config_id: 3 }],
]

beforeEach(() => {
  client.post.mockReset()
  client.post.mockResolvedValue({ data: { ok: true } })
})

describe('environment business API contracts', () => {
  it.each(cases)('%s maps to the public HTTP path', async (_name, request, suffix, body) => {
    await request()
    expect(client.post).toHaveBeenCalledWith(
      `/admin/environments/12/${suffix}`,
      { ...body, environment_id: 12 },
    )
  })

  it('rejects an invalid environment before sending a request', async () => {
    await expect(listProjects(0)).rejects.toThrow(
      'environmentId must be a positive integer',
    )
    expect(client.post).not.toHaveBeenCalled()
  })

  it('does not allow request data to replace the explicit environment', async () => {
    await createProject(12, { key: 'Demo', environment_id: 99 })

    expect(client.post).toHaveBeenCalledWith(
      '/admin/environments/12/project/create',
      { key: 'Demo', environment_id: 12 },
    )
  })
})
