// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const roleApi = vi.hoisted(() => ({
  createRole: vi.fn(),
  deleteRole: vi.fn(),
  listPermissions: vi.fn(),
  listRoles: vi.fn(),
  updateRole: vi.fn(),
  updateRolePermissions: vi.fn(),
}))

vi.mock('@/api/roles', () => roleApi)

import RolesPage from './RolesPage.vue'

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function mountPage() {
  return shallowMount(RolesPage, {
    directives: {
      loading() {},
    },
    mocks: {
      $message: { success: vi.fn() },
      $store: { getters: { 'session/systemAdmin': true } },
    },
    stubs: [
      'el-alert',
      'el-button',
      'el-checkbox',
      'el-checkbox-group',
      'el-dialog',
      'el-form',
      'el-form-item',
      'el-input',
      'el-table',
      'el-table-column',
      'el-tag',
    ],
  })
}

describe('RolesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    roleApi.listRoles.mockResolvedValue({
      list: [
        {
          id: 9,
          key: 'custom',
          name: '自定义角色',
          permissions: [{ id: 701, key: 'backend:declared' }],
        },
      ],
    })
    roleApi.listPermissions.mockResolvedValue({
      list: [
        {
          id: 701,
          key: 'backend:declared',
          name: '后端声明权限',
          description: '测试权限',
        },
      ],
    })
    roleApi.updateRolePermissions.mockResolvedValue({})
  })

  it('uses only the permission list returned by the backend', async () => {
    const wrapper = mountPage()
    await flush()

    expect(wrapper.vm.permissions).toEqual([
      {
        id: 701,
        key: 'backend:declared',
        name: '后端声明权限',
        description: '测试权限',
      },
    ])
    wrapper.vm.openPermissions(wrapper.vm.roles[0])
    expect(wrapper.vm.selectedPermissionIds).toEqual([701])

    await wrapper.vm.savePermissions()
    expect(roleApi.updateRolePermissions).toHaveBeenCalledWith(9, [701])
  })

  it('keeps the page mounted and shows an explicit error after a 403', async () => {
    roleApi.listRoles.mockRejectedValue({ response: { status: 403 } })
    const wrapper = mountPage()
    await flush()

    expect(wrapper.exists()).toBe(true)
    expect(wrapper.vm.errorMessage).toBe(
      '没有权限读取角色与权限。当前页面已保留。',
    )
  })
})
