// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import Vue from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const projectApi = vi.hoisted(() => ({
  createProject: vi.fn(),
  getProject: vi.fn(),
  updateProject: vi.fn(),
}))
const configApi = vi.hoisted(() => ({
  createConfig: vi.fn(),
  deleteConfig: vi.fn(),
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  updateConfigValue: vi.fn(),
}))
const modelApi = vi.hoisted(() => ({
  createModel: vi.fn(),
  deleteModel: vi.fn(),
  updateModel: vi.fn(),
}))
const enumApi = vi.hoisted(() => ({
  createEnum: vi.fn(),
  deleteEnum: vi.fn(),
  updateEnum: vi.fn(),
}))

vi.mock('@/api/projects', () => projectApi)
vi.mock('@/api/configs', () => configApi)
vi.mock('@/api/models', () => modelApi)
vi.mock('@/api/enums', () => enumApi)
vi.mock('@/components/MonacoEditor', () => ({
  default: { name: 'MonacoEditor', render: (createElement) => createElement('div') },
}))
vi.mock('@/components/SchemaForm', () => ({
  default: { name: 'SchemaForm', render: (createElement) => createElement('div') },
}))

import ConfigDetailPage from './ConfigDetailPage.vue'
import ConfigsPage from './ConfigsPage.vue'
import EnumsPanel from '../enums/EnumsPanel.vue'
import ModelsPanel from '../models/ModelsPanel.vue'
import ProjectsPage from '../projects/ProjectsPage.vue'

Vue.config.ignoredElements = [/^el-/]
Vue.config.productionTip = false
Vue.directive('loading', {})

const environment = { id: 11, key: 'east', name: 'East', enabled: true }

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function createMocks(dispatch = vi.fn()) {
  return {
    $store: {
      state: { environment: { currentId: 11 } },
      getters: {
        'environment/current': environment,
        'environment/hasPermission': () => true,
      },
      dispatch,
    },
    $router: { push: vi.fn(), replace: vi.fn() },
    $message: { error: vi.fn(), success: vi.fn() },
    $confirm: vi.fn(),
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  projectApi.getProject.mockResolvedValue({
    project: { id: 7, key: 'Demo', name: 'Demo' },
  })
  configApi.getConfig.mockResolvedValue({
    config: {
      id: 31,
      key: 'FeatureFlags',
      description: 'Flags',
      type: { baseType: 'STRING' },
      isArray: false,
      value: '"enabled"',
      version: 3,
    },
    tree: {
      value: {
        key: 'FeatureFlags',
        name: 'Flags',
        type: { baseType: 'STRING' },
      },
    },
  })
  projectApi.createProject.mockResolvedValue({})
  projectApi.updateProject.mockResolvedValue({})
  configApi.updateConfig.mockResolvedValue({})
  configApi.updateConfigValue.mockResolvedValue({})
  modelApi.createModel.mockResolvedValue({})
  modelApi.updateModel.mockResolvedValue({})
  enumApi.createEnum.mockResolvedValue({})
  enumApi.updateEnum.mockResolvedValue({})
})

describe('core configuration pages', () => {
  it('loads projects with an explicit environment and opens the fixed route', async () => {
    const dispatch = vi.fn().mockResolvedValue({
      list: [{ id: 7, key: 'Demo', name: 'Demo' }],
    })
    const mocks = createMocks(dispatch)
    const wrapper = shallowMount(ProjectsPage, { mocks })
    await flush()

    expect(dispatch).toHaveBeenCalledWith('configCenter/loadProjects', {
      environmentId: 11,
      filter: { keyword: '' },
      force: true,
    })
    wrapper.vm.openProject({ id: 7 })
    expect(mocks.$router.push).toHaveBeenCalledWith({
      name: 'project-configs',
      params: { projectId: '7' },
    })
  })

  it('loads project configs in the selected environment', async () => {
    const dispatch = vi.fn().mockResolvedValue({
      list: [
        {
          config: { id: 31, key: 'FeatureFlags' },
          isReleased: true,
        },
      ],
    })
    const mocks = createMocks(dispatch)
    const wrapper = shallowMount(ConfigsPage, {
      mocks,
      propsData: { projectId: 7 },
    })
    await flush()

    expect(projectApi.getProject).toHaveBeenCalledWith(11, 7)
    expect(dispatch).toHaveBeenCalledWith('configCenter/loadConfigs', {
      environmentId: 11,
      projectId: 7,
      filter: {},
      force: true,
    })
    expect(wrapper.vm.configs[0]).toMatchObject({
      id: 31,
      isReleased: true,
    })
  })

  it('clears environment-scoped configuration data after an environment change', () => {
    const reload = vi.fn()
    const context = {
      project: { id: 7 },
      configs: [{ id: 31 }],
      reload,
    }

    ConfigsPage.watch.environmentId.call(
      context,
      22,
      11,
    )

    expect(context.project).toBeNull()
    expect(context.configs).toEqual([])
    expect(reload).not.toHaveBeenCalled()
  })

  it('loads configuration resources and converts protobuf types for editors', async () => {
    const dispatch = vi.fn((action) => {
      if (action === 'configCenter/loadModels') {
        return Promise.resolve({
          list: [
            {
              id: 41,
              key: 'User',
              fields: [{ key: 'name', type: { baseType: 'STRING' } }],
            },
          ],
        })
      }
      return Promise.resolve({ list: [{ key: 'Status', values: [] }] })
    })
    const wrapper = shallowMount(ConfigDetailPage, {
      mocks: createMocks(dispatch),
      propsData: { projectId: 7, configId: 31 },
    })
    await flush()
    await Vue.nextTick()

    expect(configApi.getConfig).toHaveBeenCalledWith(11, 31)
    expect(wrapper.vm.models[0].fields[0].type).toEqual({ baseType: 'String' })
    expect(wrapper.vm.tree.value.type).toEqual({ baseType: 'String' })
    expect(wrapper.vm.previewValue).toBe('"enabled"')
  })

  it('sends edited configuration types through the public protobuf contract', async () => {
    const reload = vi.fn().mockResolvedValue()
    const context = {
      $refs: { definitionForm: { validate: (done) => done(true) } },
      $message: { error: vi.fn(), success: vi.fn() },
      environmentId: 11,
      config: { id: 31, version: 3 },
      dialog: {
        visible: true,
        saving: false,
        form: {
          description: 'Flags',
          type: JSON.stringify({ baseType: 'DatetimeRange' }),
          isArray: true,
        },
      },
      reload,
    }

    await ConfigDetailPage.methods.saveDefinition.call(context)

    expect(configApi.updateConfig).toHaveBeenCalledWith(11, {
      id: 31,
      description: 'Flags',
      type: { base_type: 'DATETIME_RANGE' },
      is_array: true,
      version: 3,
    })
    expect(reload).toHaveBeenCalled()
  })

  it('sends model fields with explicit environment and protobuf type names', async () => {
    const context = {
      $refs: { modelForm: { validate: (done) => done(true) } },
      $message: { error: vi.fn(), success: vi.fn() },
      $emit: vi.fn(),
      environmentId: 11,
      configId: 31,
      dialog: {
        visible: true,
        editing: true,
        saving: false,
        form: {
          id: 41,
          key: 'User',
          name: 'User',
          description: '',
          version: 2,
          fields: [
            {
              key: 'createdAt',
              name: 'Created at',
              description: '',
              isArray: false,
              type: JSON.stringify({ baseType: 'Datetime' }),
            },
          ],
        },
      },
      reload: vi.fn().mockResolvedValue(),
    }

    await ModelsPanel.methods.save.call(context)

    expect(modelApi.updateModel).toHaveBeenCalledWith(11, {
      id: 41,
      key: 'User',
      name: 'User',
      description: '',
      version: 2,
      fields: [
        {
          key: 'createdAt',
          name: 'Created at',
          description: '',
          is_array: false,
          type: { base_type: 'DATETIME' },
        },
      ],
    })
  })

  it('creates enums in the selected environment and current config', async () => {
    const context = {
      $refs: { enumForm: { validate: (done) => done(true) } },
      $message: { error: vi.fn(), success: vi.fn() },
      $emit: vi.fn(),
      environmentId: 11,
      configId: 31,
      dialog: {
        visible: true,
        editing: false,
        saving: false,
        form: {
          key: 'Status',
          name: 'Status',
          description: '',
          values: [{ label: 'Enabled', value: 'ENABLED', description: '' }],
        },
      },
      reload: vi.fn().mockResolvedValue(),
    }

    await EnumsPanel.methods.save.call(context)

    expect(enumApi.createEnum).toHaveBeenCalledWith(11, {
      config_id: 31,
      key: 'Status',
      name: 'Status',
      description: '',
      values: [{ label: 'Enabled', value: 'ENABLED', description: '' }],
    })
  })
})
