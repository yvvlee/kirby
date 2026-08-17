// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import Vue from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  createProjectApiKey: vi.fn(),
  listProjectApiKeys: vi.fn(),
  revokeProjectApiKey: vi.fn(),
  rotateProjectApiKey: vi.fn(),
}))

vi.mock('@/api/api-keys', () => api)

import ApiKeysPanel from './ApiKeysPanel.vue'

Vue.config.ignoredElements = [/^el-/]
Vue.config.productionTip = false
Vue.directive('loading', {})

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function mountPanel({ manage = false } = {}) {
  return shallowMount(ApiKeysPanel, {
    propsData: { visible: true, projectId: 7 },
    mocks: {
      $store: {
        state: { environment: { currentId: 11 } },
        getters: {
          'environment/hasPermission': (permission) =>
            manage && permission === 'project:api_key:manage',
        },
      },
      $message: { error: vi.fn(), success: vi.fn() },
      $confirm: vi.fn(),
    },
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.listProjectApiKeys.mockResolvedValue({ list: [] })
  api.createProjectApiKey.mockResolvedValue({
    apiKey: { id: 31, publicId: 'pub_31' },
    secret: 'one-time-secret',
  })
})

describe('project API Key management', () => {
  it('shows management actions only with the backend permission', async () => {
    const reader = mountPanel()
    const manager = mountPanel({ manage: true })
    await flush()

    expect(reader.vm.canManage).toBe(false)
    expect(manager.vm.canManage).toBe(true)
    expect(() => reader.vm.openCreate()).toThrow(
      '当前用户没有管理项目 API Key 的权限',
    )
  })

  it('keeps a complete secret only until copy acknowledgement closes', () => {
    const storageSpy = vi.spyOn(Storage.prototype, 'setItem')
    const context = {
      secretDialog: {},
    }

    ApiKeysPanel.methods.showSecret.call(context, {
      apiKey: { id: 31 },
      secret: 'one-time-secret',
    })
    expect(context.secretDialog).toMatchObject({
      visible: true,
      secret: 'one-time-secret',
      acknowledged: false,
    })
    expect(() =>
      ApiKeysPanel.methods.confirmSecretCopied.call(context),
    ).toThrow('必须确认已经复制并保存 Secret')

    context.secretDialog.acknowledged = true
    ApiKeysPanel.methods.confirmSecretCopied.call(context)
    ApiKeysPanel.methods.clearSecret.call(context)
    expect(context.secretDialog.secret).toBeNull()
    expect(storageSpy).not.toHaveBeenCalled()
    storageSpy.mockRestore()
  })

  it('rejects a list response that leaks complete secrets', async () => {
    api.listProjectApiKeys.mockResolvedValue({
      list: [{ id: 31, secret: 'must-not-be-listed' }],
    })
    const wrapper = mountPanel()
    await flush()

    expect(wrapper.vm.keys).toEqual([])
    expect(wrapper.vm.errorMessage).toBe(
      'API Key 列表不得包含完整 Secret',
    )
  })

  it('shows an explicit error after a backend 403', async () => {
    api.listProjectApiKeys.mockRejectedValue({ response: { status: 403 } })
    const wrapper = mountPanel()
    await flush()

    expect(wrapper.vm.errorMessage).toBe(
      '没有权限读取项目 API Key。当前页面已保留。',
    )
  })
})
